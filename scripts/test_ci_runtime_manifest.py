import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("manifest", Path(__file__).with_name("ci_runtime_manifest.py"))
manifest = importlib.util.module_from_spec(spec)
spec.loader.exec_module(manifest)


class RuntimeManifestTests(unittest.TestCase):
    def test_source_platform_bytes_and_complete_generated_file_set_must_match(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(manifest, "source_id", return_value="source-a"):
            root = Path(directory)
            binary = root / "provider"
            binary.write_bytes(b"compiled runtime")
            crds = root / "crds"
            crds.mkdir()
            (crds / "resource.yaml").write_text("kind: CustomResourceDefinition\n")
            saved = root / "manifest.json"
            saved.write_text(json.dumps(manifest.snapshot([binary, crds])))
            manifest.check(saved, [binary, crds])
            for field, value in (("source", "source-b"), ("platform", ["other", "arm64"])):
                original = json.loads(saved.read_text())
                saved.write_text(json.dumps({**original, field: value}))
                with self.assertRaises(ValueError):
                    manifest.check(saved, [binary, crds])
                saved.write_text(json.dumps(original))
            binary.write_bytes(b"different runtime")
            with self.assertRaises(ValueError):
                manifest.check(saved, [binary, crds])
            binary.write_bytes(b"compiled runtime")
            extra = crds / "stale.yaml"
            extra.write_text("stale checkout output")
            with self.assertRaises(ValueError):
                manifest.check(saved, [binary, crds])
            extra.unlink()
            (crds / "resource.yaml").unlink()
            with self.assertRaises(ValueError):
                manifest.check(saved, [binary, crds])

    def test_empty_outputs_and_symlinks_cannot_be_verified(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            empty = root / "empty"
            empty.touch()
            with self.assertRaises(ValueError):
                manifest.snapshot([empty])
            link = root / "link"
            link.symlink_to(root, target_is_directory=True)
            with self.assertRaises(ValueError):
                manifest.snapshot([link])


if __name__ == "__main__":
    unittest.main()
