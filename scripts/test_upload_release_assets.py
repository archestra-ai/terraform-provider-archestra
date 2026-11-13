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


class Gpg:
    """A fake gpg: the signature is valid, by SIGNER, when it holds the sums' sha256."""

    signer = upload.SIGNING_KEY

    def run(self, args, **kwargs):
        if "--import" in args:
            return subprocess.CompletedProcess(args, 0, "", "")
        signature, sums = Path(args[-2]), Path(args[-1])
        if signature.read_bytes() != hashlib.sha256(sums.read_bytes()).hexdigest().encode():
            return subprocess.CompletedProcess(args, 1, "[GNUPG:] BADSIG x\n", "")
        status = f"[GNUPG:] VALIDSIG {self.signer} 2026-11-08 1 0 4 0 1 10 00 {self.signer}\n"
        return subprocess.CompletedProcess(args, 0, status, "")


GPG = Gpg()


class Gh:
    """A fake gh holding one release."""

    def __init__(self, assets=None, draft=False, exists=True):
        self.assets = dict(assets or {})
        self.draft = draft
        self.exists = exists
        self.calls = []

    def run(self, args, **kwargs):
        if args[0] == "gpg":
            return GPG.run(args, **kwargs)
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
        GPG.signer = upload.SIGNING_KEY
        self.directory = Path(tempfile.mkdtemp())
        self.built = Path(tempfile.mkdtemp())
        self.files = {f"{PREFIX}_linux_amd64.zip": b"zip", f"{PREFIX}_manifest.json": b"manifest"}
        sums = "".join(f"{hashlib.sha256(body).hexdigest()}  {name}\n" for name, body in sorted(self.files.items()))
        self.files[f"{PREFIX}_SHA256SUMS"] = sums.encode()
        for name, body in self.files.items():
            (self.directory / name).write_bytes(body)
            (self.built / name).write_bytes(body)
        self.files[f"{PREFIX}_SHA256SUMS.sig"] = hashlib.sha256(sums.encode()).hexdigest().encode()
        (self.directory / f"{PREFIX}_SHA256SUMS.sig").write_bytes(self.files[f"{PREFIX}_SHA256SUMS.sig"])

    def upload(self, gh, *extra):
        with mock.patch.object(upload.subprocess, "run", side_effect=gh.run):
            upload.main([str(self.directory), "v3.1.3", "--built", str(self.built), *extra], ENVIRONMENT)

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

    def test_altered_or_badly_signed_assets_upload_nothing(self):
        zip_name = f"{PREFIX}_linux_amd64.zip"
        cases = {
            "zip changed after signing": lambda: (self.directory / zip_name).write_bytes(b"tampered"),
            "zip and sums changed, old signature": lambda: self.rewrite_sums(zip_name, b"tampered"),
            "signed by another key": lambda: setattr(GPG, "signer", "A" * 40),
            "not the file this run built": lambda: (self.built / zip_name).write_bytes(b"other build"),
            "a file this run did not build": lambda: (self.directory / f"{PREFIX}_extra.zip").write_bytes(b"x"),
        }
        for name, alter in cases.items():
            with self.subTest(name):
                self.setUp()
                alter()
                gh = Gh()
                with self.assertRaises(ValueError):
                    self.upload(gh)
                self.assertEqual(gh.calls, [])

    def rewrite_sums(self, name, body):
        (self.directory / name).write_bytes(body)
        files = {path.name: path.read_bytes() for path in self.directory.iterdir()
                 if not path.name.endswith(("SHA256SUMS", ".sig"))}
        (self.directory / f"{PREFIX}_SHA256SUMS").write_text(
            "".join(f"{hashlib.sha256(b).hexdigest()}  {n}\n" for n, b in sorted(files.items())))

    def test_assets_without_a_signature_are_refused(self):
        (self.directory / f"{PREFIX}_SHA256SUMS.sig").unlink()
        gh = Gh()
        with self.assertRaisesRegex(ValueError, "signature"):
            self.upload(gh)
        self.assertEqual(gh.uploaded(), [])


if __name__ == "__main__":
    unittest.main()
