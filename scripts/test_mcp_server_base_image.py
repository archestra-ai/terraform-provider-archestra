import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('mcp_image', Path(__file__).with_name('mcp-server-base-image.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class CompanionImageTests(unittest.TestCase):
    def test_published_registry_boundaries(self):
        for version in ['1.4.0-rc.51', '1.4.0', '1.4.7', '1.5.0-rc.6', '1.5.0-rc.11']:
            with self.subTest(version=version):
                self.assertEqual(module.image(version), 'europe-west1-docker.pkg.dev/friendly-path-465518-r6/archestra-public/mcp-server-base:' + version)
        for version in ['1.4.8', '1.4.10', '1.5.0-rc.12', '1.5.0-rc.13', '1.5.0', '2.0.0']:
            with self.subTest(version=version):
                self.assertEqual(module.image(version), 'ghcr.io/archestra-ai/mcp-server-base:' + version)

    def test_invalid_versions_fail(self):
        for version in ['', 'latest', '1.5.0;echo invalid', '1.5.0/other', '1.5.0\n']:
            with self.subTest(version=version), self.assertRaises(ValueError):
                module.image(version)
