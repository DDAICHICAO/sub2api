#!/usr/bin/env python3
"""Install the optional host updater without restarting the application."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess


def run(*args):
    return subprocess.check_output(args)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--container', default='sub2api')
    parser.add_argument('--database-container', default='sub2api-postgres')
    parser.add_argument('--health-url', default='http://127.0.0.1:8080/health')
    parser.add_argument('--settings-url', default='http://127.0.0.1:8080/api/v1/settings/public')
    args = parser.parse_args()
    if os.geteuid() != 0:
        raise SystemExit('Run as root on the Docker host')
    root = Path('/etc/sub2api-updater')
    if (root / 'config.json').exists():
        raise SystemExit('Updater already installed; preserve its config when updating updater.py')
    item = json.loads(run('docker', 'inspect', args.container))[0]
    labels = item['Config']['Labels']
    if labels.get('com.docker.compose.service') != 'sub2api':
        raise SystemExit('Expected Compose application service sub2api')
    files = labels['com.docker.compose.project.config_files'].split(',')
    working_dir = labels['com.docker.compose.project.working_dir']
    project = labels['com.docker.compose.project']
    command = ['docker', 'compose', '--project-directory', working_dir, '-p', project]
    for file in files:
        command += ['-f', file]
    # Use effective Compose data, preserving expanded environment, paths and
    # project identity. Do not read a guessed default docker-compose.yml.
    effective = json.loads(run(*command, 'config', '--format', 'json'))
    app = effective['services']['sub2api']
    if app.get('image') != item['Config']['Image']:
        raise SystemExit('Effective Compose image differs from running container')
    mounts = [m for m in item['Mounts'] if m['Destination'] == '/app/data' and m['Type'] == 'bind']
    if len(mounts) != 1:
        raise SystemExit('A bind mount at /app/data is required')
    database = json.loads(run('docker', 'inspect', args.database_container))[0]
    env = dict(value.split('=', 1) for value in database['Config']['Env'] if '=' in value)
    socket_dir = Path(mounts[0]['Source']) / '.updater'
    if socket_dir.is_symlink():
        raise SystemExit('Refusing symlink socket directory')
    socket_dir.mkdir(mode=0o755, exist_ok=True)
    os.chown(socket_dir, 0, 0)
    os.chmod(socket_dir, 0o755)
    root.mkdir(mode=0o700, exist_ok=True)
    state = Path('/var/lib/sub2api-updater')
    state.mkdir(mode=0o700, exist_ok=True)
    # Keep a root-owned JSON Compose file in place when one is already used,
    # as on migrated installations. Otherwise establish a managed snapshot.
    if len(files) == 1 and Path(files[0]).suffix == '.json':
        compose = Path(files[0])
        if compose.is_symlink() or compose.stat().st_uid != 0 or compose.stat().st_mode & 0o022:
            raise SystemExit('Compose file must be root-owned and not writable by other users')
        shutil.copy2(compose, root / 'compose.before-install.json')
    else:
        compose = root / 'compose.json'
        compose.write_text(json.dumps(effective, indent=2))
        os.chmod(compose, 0o600)
    # Discover the application group instead of assuming a numeric UID/GID.
    app_gid = int(run('docker', 'exec', args.container, 'id', '-g', 'sub2api').decode().strip())
    config = {'compose_file': str(compose), 'working_dir': working_dir, 'project': project,
              'container': args.container, 'database_container': args.database_container,
              'database_user': env.get('POSTGRES_USER', 'postgres'),
              'database_name': env.get('POSTGRES_DB', env.get('POSTGRES_USER', 'postgres')),
              'state_dir': str(state), 'socket': str(socket_dir / 'control.sock'),
              'socket_gid': app_gid, 'health_url': args.health_url, 'settings_url': args.settings_url}
    path = root / 'config.json'
    path.write_text(json.dumps(config, indent=2))
    os.chmod(path, 0o600)
    shutil.copyfile(Path(__file__).with_name('updater.py'), root / 'updater.py')
    os.chmod(root / 'updater.py', 0o600)
    unit = '''[Unit]
Description=Sub2API host image updater
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/bin/python3 /etc/sub2api-updater/updater.py /etc/sub2api-updater/config.json
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
TimeoutStopSec=10

[Install]
WantedBy=multi-user.target
'''
    Path('/etc/systemd/system/sub2api-updater.service').write_text(unit)
    subprocess.run(['systemctl', 'daemon-reload'], check=True)
    subprocess.run(['systemctl', 'enable', '--now', 'sub2api-updater.service'], check=True)
    print('Host updater installed. Application was not restarted. Managed Compose:', compose)


if __name__ == '__main__':
    main()
