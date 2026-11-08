"""Exercise the release asset upload, stubbing only the gh process boundary."""

import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("upload", Path(__file__).with_name("upload-release-assets.py"))
upload = importlib.util.module_from_spec(spec)
spec.loader.exec_module(upload)

PREFIX = "terraform-provider-archestra_3.1.3"
ENVIRONMENT = {"GITHUB_REPOSITORY": "archestra-ai/terraform-provider-archestra"}


def digest(body):
    return "sha256:" + hashlib.sha256(body).hexdigest()


class Gh:
    """A fake gh holding one release."""

    def __init__(self, assets=None, draft=False, exists=True):
        self.assets = dict(assets or {})
        self.draft = draft
        self.exists = exists
        self.calls = []

    def run(self, args, **kwargs):
        self.calls.append(args)
        if args[1:3] == ["release", "create"]:
            self.exists, self.draft = True, "--draft" in args
            return subprocess.CompletedProcess(args, 0, "", "")
        if args[1:3] == ["release", "view"]:
            if not self.exists:
                raise subprocess.CalledProcessError(1, args, "", "release not found")
            assets = [{"name": name, "digest": value} for name, value in self.assets.items()]
            return subprocess.CompletedProcess(args, 0, json.dumps({"isDraft": self.draft, "assets": assets}), "")
        if args[1:3] == ["release", "upload"]:
            path = Path(args[4])
            self.assets[path.name] = digest(path.read_bytes())
            return subprocess.CompletedProcess(args, 0, "", "")
        raise AssertionError(f"Unexpected command {args}")

    def uploaded(self):
        return [Path(call[4]).name for call in self.calls if call[1:3] == ["release", "upload"]]


class UploadTests(unittest.TestCase):
    def setUp(self):
        self.directory = Path(tempfile.mkdtemp())
        self.files = {f"{PREFIX}_linux_amd64.zip": b"zip", f"{PREFIX}_manifest.json": b"manifest",
                      f"{PREFIX}_SHA256SUMS": b"sums", f"{PREFIX}_SHA256SUMS.sig": b"signature"}
        for name, body in self.files.items():
            (self.directory / name).write_bytes(body)

    def upload(self, gh, *extra):
        with mock.patch.object(upload.subprocess, "run", side_effect=gh.run):
            upload.main([str(self.directory), "v3.1.3", *extra], ENVIRONMENT)

    def test_uploads_every_file_with_the_signature_last(self):
        gh = Gh()
        self.upload(gh)
        self.assertEqual(gh.uploaded()[-1], f"{PREFIX}_SHA256SUMS.sig")
        self.assertEqual(set(gh.uploaded()), set(self.files))

    def test_a_retry_skips_what_the_release_holds_and_keeps_its_signature(self):
        held = {name: digest(body) for name, body in self.files.items() if not name.endswith("manifest.json")}
        held[f"{PREFIX}_SHA256SUMS.sig"] = digest(b"an earlier valid signature")
        gh = Gh(assets=held)
        self.upload(gh)
        self.assertEqual(gh.uploaded(), [f"{PREFIX}_manifest.json"])

    def test_another_file_under_the_same_name_writes_nothing(self):
        gh = Gh(assets={f"{PREFIX}_manifest.json": digest(b"other")})
        with self.assertRaisesRegex(ValueError, "immutable"):
            self.upload(gh)
        self.assertEqual(gh.uploaded(), [])

    def test_a_signature_without_its_sums_writes_nothing(self):
        gh = Gh(assets={f"{PREFIX}_SHA256SUMS.sig": digest(b"signature")})
        with self.assertRaisesRegex(ValueError, "signature but not"):
            self.upload(gh)
        self.assertEqual(gh.uploaded(), [])

    def test_a_dry_run_uploads_to_a_new_draft_only(self):
        gh = Gh(exists=False)
        self.upload(gh, "--draft-for-smoke", "f" * 40)
        self.assertTrue(gh.draft)
        self.assertEqual(set(gh.uploaded()), set(self.files))
        # A release run never uploads to a draft, and a dry run never to a published release.
        with self.assertRaisesRegex(ValueError, "must not be a draft"):
            self.upload(Gh(draft=True))

    def test_assets_without_a_signature_are_refused(self):
        (self.directory / f"{PREFIX}_SHA256SUMS.sig").unlink()
        gh = Gh()
        with self.assertRaisesRegex(ValueError, "signature"):
            self.upload(gh)
        self.assertEqual(gh.uploaded(), [])


if __name__ == "__main__":
    unittest.main()
