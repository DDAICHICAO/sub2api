import importlib.util
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('updater', Path(__file__).with_name('updater.py'))
u = importlib.util.module_from_spec(spec)
spec.loader.exec_module(u)


class UpdaterTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.compose = self.root / 'compose.json'
        u.atomic_json(self.compose, {'services': {'sub2api': {'image': u.IMAGE + ':0.2.17'}, 'postgres': {'image': 'postgres:18'}}})
        self.app = u.Updater({'state_dir': str(self.root / 'state'), 'compose_file': str(self.compose), 'project': 'test', 'working_dir': str(self.root), 'container': 'sub2api', 'database_container': 'postgres', 'database_user': 'app', 'database_name': 'app'})
        guard = patch.object(self.app, 'protected_state', return_value={'environment': ['EXAMPLE=preserved']})
        guard.start()
        self.addCleanup(guard.stop)

    def test_reject_arbitrary_image_and_shell_input(self):
        for value in ['latest', '0.2.18;id', '../x', 'evil/image:0.2.18', 'v0.2.18', '', None]:
            with self.assertRaises(ValueError):
                self.app.start(value)

    def test_single_flight_and_same_target_is_idempotent(self):
        with patch.object(u.threading.Thread, 'start'):
            first = self.app.start('0.2.18')
            self.assertEqual(first['job_id'], self.app.start('0.2.18')['job_id'])
            with self.assertRaises(u.Busy):
                self.app.start('0.2.19')

    def test_backup_failure_never_replaces_container_or_compose(self):
        old = self.compose.read_bytes()
        with patch.object(self.app, 'prepare_image'), patch.object(self.app, 'backup', side_effect=RuntimeError('backup failed')), patch.object(self.app, 'recreate') as recreate:
            self.app.state = {'active': True, 'version': '0.2.18', 'job_id': 'test'}
            self.app.run('0.2.18')
        recreate.assert_not_called()
        self.assertEqual(old, self.compose.read_bytes())
        self.assertEqual('failed', self.app.state['stage'])

    def test_failed_candidate_restores_config_and_image_without_database_restore(self):
        old = self.compose.read_bytes()
        with patch.object(self.app, 'prepare_image'), patch.object(self.app, 'backup', return_value=self.root), patch.object(self.app, 'recreate') as recreate, patch.object(self.app, 'verify', side_effect=[RuntimeError('unhealthy'), None]):
            self.app.state = {'active': True, 'version': '0.2.18', 'job_id': 'test'}
            self.app.run('0.2.18')
        self.assertEqual(2, recreate.call_count)
        self.assertEqual(old, self.compose.read_bytes())
        self.assertEqual('rolled_back', self.app.state['stage'])

    def test_success_changes_only_application_image(self):
        with patch.object(self.app, 'prepare_image'), patch.object(self.app, 'backup', return_value=self.root), patch.object(self.app, 'recreate'), patch.object(self.app, 'verify'):
            self.app.state = {'active': True, 'version': '0.2.18', 'job_id': 'test'}
            self.app.run('0.2.18')
        data = u.json.loads(self.compose.read_text())
        self.assertEqual('postgres:18', data['services']['postgres']['image'])
        self.assertEqual(u.IMAGE + ':0.2.18', data['services']['sub2api']['image'])
        self.assertEqual('completed', self.app.state['stage'])

    def test_environment_drift_triggers_image_rollback(self):
        with patch.object(self.app, 'prepare_image'), patch.object(self.app, 'backup', return_value=self.root), patch.object(self.app, 'recreate'), patch.object(self.app, 'verify'), patch.object(self.app, 'protected_state', side_effect=[{'a': 1}, {'a': 2}, {'a': 1}]):
            self.app.state = {'active': True, 'version': '0.2.18', 'job_id': 'test'}
            self.app.run('0.2.18')
        self.assertEqual('rolled_back', self.app.state['stage'])

    def test_restart_keeps_job_identity_but_marks_interrupted(self):
        with patch.object(u.threading.Thread, 'start'):
            job = self.app.start('0.2.18')
        restarted = u.Updater(self.app.config)
        self.assertEqual(job['job_id'], restarted.state['job_id'])
        self.assertEqual('interrupted', restarted.state['stage'])
        self.assertFalse(restarted.state['active'])

    def test_unpublished_or_wrong_identity_image_is_rejected(self):
        with patch.object(u, 'github', return_value={'draft': True, 'prerelease': False, 'tag_name': 'v0.2.18'}), patch.object(u, 'command') as command:
            with self.assertRaises(ValueError):
                self.app.prepare_image('0.2.18')
            command.assert_not_called()
        responses = [{'draft': False, 'prerelease': False, 'tag_name': 'v0.2.18'}, {'object': {'type': 'commit', 'sha': 'expected'}}]
        wrong_image = u.json.dumps([{'Config': {'Labels': {'org.opencontainers.image.version': '0.2.18', 'org.opencontainers.image.revision': 'wrong', 'org.opencontainers.image.source': 'https://github.com/' + u.REPOSITORY}}}]).encode()
        with patch.object(u, 'github', side_effect=responses), patch.object(u, 'command', side_effect=[b'', wrong_image]):
            with self.assertRaisesRegex(ValueError, 'identity mismatch'):
                self.app.prepare_image('0.2.18')


if __name__ == '__main__':
    unittest.main()
