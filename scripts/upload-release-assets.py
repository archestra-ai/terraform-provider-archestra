#!/usr/bin/env python3
"""Upload the publisher's signed release assets to this repository's release.

archestra-ai/release-publisher verifies the goreleaser files this run built,
signs SHA256SUMS with the provider's key and returns the verified files and
SHA256SUMS.sig as its release-assets artifact. This uploads exactly those
files to the GitHub release the Terraform Registry reads.

Assets are immutable: a file the release already holds with the same sha256
is skipped, and one it holds with another sha256 stops the upload before
anything is written. A signature is never compared byte for byte (signing the
same file twice gives two valid signatures), so an existing SHA256SUMS.sig is
kept when the release already holds this SHA256SUMS. The signature goes last,
after every file it vouches for.

Usage: upload-release-assets.py <assets directory> <tag> [--draft-for-smoke <commit>]
With --draft-for-smoke, the release is a new draft named after the tag, which
the Terraform Registry never reads.
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path


def main(argv, environment):
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("tag")
    parser.add_argument("--draft-for-smoke", metavar="COMMIT")
    args = parser.parse_args(argv)
    repository = environment["GITHUB_REPOSITORY"]
    files = sorted(path for path in args.directory.iterdir() if path.is_file())
    signatures = [path for path in files if path.name.endswith("_SHA256SUMS.sig")]
    if len(signatures) != 1 or not (args.directory / signatures[0].name.removesuffix(".sig")).is_file():
        raise ValueError("The assets must hold one SHA256SUMS and its signature")
    signature = signatures[0]
    sums = args.directory / signature.name.removesuffix(".sig")
    if args.draft_for_smoke:
        gh("release", "create", args.tag, "--repo", repository, "--draft", "--target", args.draft_for_smoke,
           "--title", f"Publisher dry run {args.tag}", "--notes", "Publisher dry run. Not a release.")
    release = json.loads(gh("release", "view", args.tag, "--repo", repository, "--json", "isDraft,assets"))
    if bool(args.draft_for_smoke) != release["isDraft"]:
        raise ValueError(f"Release {args.tag} must {'' if args.draft_for_smoke else 'not '}be a draft")
    existing = {asset["name"]: asset.get("digest") for asset in release["assets"]}
    uploads = []
    for path in [path for path in files if path != signature]:
        digest = "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()
        if path.name not in existing:
            uploads.append(path)
        elif existing[path.name] != digest:
            raise ValueError(f"{args.tag} already holds another {path.name}; release assets are immutable")
    if signature.name not in existing:
        uploads.append(signature)
    elif sums in uploads:
        raise ValueError(f"{args.tag} holds a signature but not this {sums.name}")
    for path in uploads:
        gh("release", "upload", args.tag, str(path), "--repo", repository)
        print(f"Uploaded {path.name}")
    if not uploads:
        print(f"{args.tag} already holds every asset")


def gh(*args):
    return subprocess.run(["gh", *args], check=True, capture_output=True, text=True).stdout


if __name__ == "__main__":
    try:
        main(sys.argv[1:], os.environ)
    except (KeyError, ValueError, json.JSONDecodeError, subprocess.CalledProcessError) as error:
        detail = getattr(error, "stderr", None)
        sys.exit(f"::error::Release asset upload failed: {error}" + (f"\n{detail}" if detail else ""))
