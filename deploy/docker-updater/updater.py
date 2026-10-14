#!/usr/bin/env python3
"""Host-owned, single-purpose Compose updater. No Docker socket in the app."""
import copy
import http.server
import json
import logging
import os
from pathlib import Path
import re
import socketserver
import subprocess
import threading
import time
import urllib.request
import uuid

IMAGE = 'ghcr.io/ddaichicao/sub2api'
REPOSITORY = 'DDAICHICAO/sub2api'
VERSION = re.compile(r'0\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\Z')


def atomic_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix('.tmp')
    with open(temp, 'w', encoding='utf-8') as out:
        os.chmod(temp, 0o600)
        json.dump(value, out, indent=2)
        out.flush()
        os.fsync(out.fileno())
    os.replace(temp, path)


def command(args, timeout=900):
    result = subprocess.run(args, capture_output=True, timeout=timeout, check=False)
    if result.returncode:
        logging.error('command %s failed: %s', args[:3], result.stderr.decode(errors='replace')[-2000:])
        raise RuntimeError('host command failed; inspect updater journal')
    return result.stdout


def github(path):
    request = urllib.request.Request('https://api.github.com/repos/' + REPOSITORY + '/' + path,
                                     headers={'Accept': 'application/vnd.github+json', 'User-Agent': 'sub2api-host-updater'})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


class Busy(Exception):
    pass


class Updater:
    def __init__(self, config):
        self.config = config
        self.compose = Path(config['compose_file'])
        self.state_dir = Path(config['state_dir'])
        self.state_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.lock = threading.Lock()
        state_file = self.state_dir / 'status.json'
        self.state = json.loads(state_file.read_text()) if state_file.exists() else {'stage': 'idle', 'active': False}
        if self.state.get('active'):
            # Never silently restart an interrupted mutation or restore a DB.
            self.set_stage('interrupted', active=False)

    def status(self):
        with self.lock:
            return dict(self.state, available=True)

    def set_stage(self, stage, **fields):
        with self.lock:
            self.state.update(stage=stage, updated_at=int(time.time()), **fields)
            atomic_json(self.state_dir / 'status.json', self.state)

    def start(self, version):
        if not isinstance(version, str) or not VERSION.fullmatch(version):
            raise ValueError('invalid version')
        with self.lock:
            if self.state.get('active'):
                if self.state.get('version') == version:
                    return dict(self.state, available=True)
                raise Busy('an update is already running')
            self.state = {'job_id': str(uuid.uuid4()), 'version': version, 'stage': 'queued', 'active': True, 'updated_at': int(time.time())}
            atomic_json(self.state_dir / 'status.json', self.state)
            result = dict(self.state, available=True)
        threading.Thread(target=self.run, args=(version,), daemon=True).start()
        return result

    def compose_command(self, *args):
        return ['docker', 'compose', '--project-directory', self.config['working_dir'], '-p', self.config['project'], '-f', str(self.compose), *args]

    def prepare_image(self, version):
        release = github('releases/tags/v' + version)
        if release['draft'] or release['prerelease'] or release['tag_name'] != 'v' + version:
            raise ValueError('only stable fork releases can be installed')
        ref = github('git/ref/tags/v' + version)['object']
        for _ in range(3):
            if ref['type'] == 'commit':
                break
            if ref['type'] != 'tag':
                raise ValueError('invalid release tag')
            ref = github('git/tags/' + ref['sha'])['object']
        if ref['type'] != 'commit':
            raise ValueError('invalid release commit')
        image = IMAGE + ':' + version
        command(['docker', 'pull', image])
        info = json.loads(command(['docker', 'image', 'inspect', image]))[0]
        labels = info['Config'].get('Labels', {})
        if labels.get('org.opencontainers.image.version') != version or labels.get('org.opencontainers.image.revision') != ref['sha'] or labels.get('org.opencontainers.image.source') != 'https://github.com/' + REPOSITORY:
            raise ValueError('image release identity mismatch')
        output = command(['docker', 'run', '--rm', '--network', 'none', '--entrypoint', '/app/sub2api', image, '--version'], 60).decode()
        if not re.search(r'(?<![\d.])' + re.escape(version) + r'(?![\d.])', output):
            raise ValueError('image binary version mismatch')

    def backup(self):
        directory = self.state_dir / 'backups' / self.state['job_id']
        directory.mkdir(parents=True, mode=0o700)
        atomic_json(directory / 'compose.json', json.loads(self.compose.read_text()))
        db = self.config
        with open(directory / 'database.dump', 'wb') as out:
            os.chmod(directory / 'database.dump', 0o600)
            result = subprocess.run(['docker', 'exec', db['database_container'], 'pg_dump', '-U', db['database_user'], '-d', db['database_name'], '-Fc'], stdout=out, stderr=subprocess.PIPE, timeout=900)
        if result.returncode:
            raise RuntimeError('database backup failed')
        with open(directory / 'database.dump', 'rb') as source:
            result = subprocess.run(['docker', 'exec', '-i', db['database_container'], 'pg_restore', '--list'], stdin=source, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=120)
        if result.returncode:
            raise RuntimeError('database backup verification failed')
        return directory

    def recreate(self):
        command(self.compose_command('up', '-d', '--no-deps', '--pull', 'never', 'sub2api'), 300)

    def protected_state(self):
        item = json.loads(command(['docker', 'inspect', self.config['container']], 15))[0]
        expected = json.loads(self.compose.read_text())['services']['sub2api']
        if item['Config']['Image'] != expected['image']:
            raise RuntimeError('running image differs from managed Compose')
        ids = command(['docker', 'ps', '-aq', '--filter', 'label=com.docker.compose.project=' + self.config['project']]).decode().split()
        others = {}
        for container_id in ids:
            container = json.loads(command(['docker', 'inspect', container_id]))[0]
            if container['Name'].lstrip('/') != self.config['container']:
                others[container['Name']] = container['Id']
        return {
            'mounts': sorted((m['Type'], m['Source'], m['Destination'], m['RW']) for m in item['Mounts']),
            'environment': sorted(item['Config']['Env']),
            'ports': item['HostConfig']['PortBindings'],
            'other_containers': others,
        }

    def verify(self, version):
        deadline = time.monotonic() + 180
        while time.monotonic() < deadline:
            try:
                item = json.loads(command(['docker', 'inspect', self.config['container']], 15))[0]
                if item['Config']['Image'] != IMAGE + ':' + version or item['State']['Status'] != 'running':
                    raise RuntimeError('container not ready')
                # Probe the application locally, without Cloudflare or admin tokens.
                with urllib.request.urlopen(self.config['health_url'], timeout=5) as response:
                    health = json.load(response)
                if health.get('status') != 'ok':
                    raise RuntimeError('application not healthy')
                with urllib.request.urlopen(self.config['settings_url'], timeout=5) as response:
                    settings = json.load(response)
                if settings.get('data', {}).get('version') != version:
                    raise RuntimeError('application version mismatch')
                return
            except Exception:
                time.sleep(3)
        raise RuntimeError('application readiness/version verification failed')

    def run(self, version):
        old = None
        changed = False
        try:
            self.set_stage('pulling')
            self.prepare_image(version)
            old = json.loads(self.compose.read_text())
            old_image = old['services']['sub2api']['image']
            if not old_image.startswith(IMAGE + ':') or not VERSION.fullmatch(old_image.split(':')[-1]):
                raise ValueError('current image is not a pinned fork release')
            if tuple(map(int, version.split('.'))) <= tuple(map(int, old_image.split(':')[-1].split('.'))):
                raise ValueError('target must be newer than the current version')
            protected = self.protected_state()
            self.set_stage('backing_up')
            self.backup()
            candidate = copy.deepcopy(old)
            candidate['services']['sub2api']['image'] = IMAGE + ':' + version
            self.set_stage('recreating')
            atomic_json(self.compose, candidate)
            changed = True
            self.recreate()
            self.set_stage('verifying')
            self.verify(version)
            if self.protected_state() != protected:
                raise RuntimeError('deployment environment, mounts or neighboring containers changed')
            self.set_stage('completed', active=False)
        except Exception:
            logging.exception('update failed at %s', self.state.get('stage'))
            if changed and old is not None:
                try:
                    self.set_stage('rolling_back')
                    atomic_json(self.compose, old)
                    self.recreate()
                    self.verify(old['services']['sub2api']['image'].split(':')[-1])
                    if self.protected_state() != protected:
                        raise RuntimeError('deployment state not restored')
                    self.set_stage('rolled_back', active=False)
                    return
                except Exception:
                    logging.exception('image rollback failed; backup retained')
            self.set_stage('failed', active=False)


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == '/status':
            self.reply(200, self.server.updater.status())
        else:
            self.reply(404, {'error': 'not found'})

    def do_POST(self):
        try:
            self.connection.settimeout(5)
            length = int(self.headers.get('Content-Length', '0'))
            if self.path != '/update' or length < 1 or length > 256:
                raise ValueError('invalid request')
            data = json.loads(self.rfile.read(length))
            if not isinstance(data, dict) or set(data) != {'version'}:
                raise ValueError('invalid request')
            self.reply(202, self.server.updater.start(data['version']))
        except Busy:
            self.reply(409, {'error': 'update already running'})
        except (ValueError, TypeError):
            self.reply(400, {'error': 'invalid update request'})


def main():
    class Server(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
        daemon_threads = True

    import fcntl
    import sys
    config = json.loads(Path(sys.argv[1]).read_text())
    lock = open(Path(config['state_dir']) / 'service.lock', 'w')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    updater = Updater(config)
    path = Path(config['socket'])
    if path.is_symlink() or path.parent.is_symlink():
        raise RuntimeError('invalid socket path')
    path.unlink(missing_ok=True)
    with Server(str(path), Handler) as server:
        os.chown(path, 0, config['socket_gid'])
        os.chmod(path, 0o660)
        server.updater = updater
        server.serve_forever()


if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    main()
