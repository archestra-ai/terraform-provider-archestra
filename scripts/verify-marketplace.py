#!/usr/bin/env python3
"""Verify anonymous Marketplace publication, including the rendered SVG logo."""

import argparse
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import time
from urllib.request import urlopen
import xml.etree.ElementTree as ET

ORIGIN = "https://marketplace.upbound.io"
ACCOUNT = "archestra"
PACKAGE = "provider-archestra"
ICON = Path(__file__).resolve().parents[1] / "crossplane/marketplace/icons/icon.svg"


class PageData(HTMLParser):
    def __init__(self):
        super().__init__()
        self.inside = False
        self.parts = []

    def handle_starttag(self, tag, attrs):
        if tag == "script" and dict(attrs).get("id") == "__NEXT_DATA__":
            self.inside = True

    def handle_endtag(self, tag):
        if tag == "script":
            self.inside = False

    def handle_data(self, data):
        if self.inside:
            self.parts.append(data)


def svg_shape(content):
    if b"<!DOCTYPE" in content.upper():
        raise ValueError("SVG must not contain a DOCTYPE")
    root = ET.fromstring(content)
    if root.tag != "{http://www.w3.org/2000/svg}svg":
        raise ValueError("Marketplace returned a non-SVG icon")

    def shape(node):
        # Marketplace may resize the root. Preserve the viewBox and all artwork.
        attrs = {key: value for key, value in node.attrib.items()
                 if node is not root or key not in ("width", "height")}
        return node.tag, sorted(attrs.items()), (node.text or "").strip(), [shape(child) for child in node]

    return shape(root)


def verify(html, icon, version, digest, expected_icon):
    parser = PageData()
    parser.feed(html.decode("utf-8"))
    data = json.loads("".join(parser.parts))
    expected_query = {"packageType": "providers", "packageAccount": ACCOUNT,
                      "packageName": PACKAGE, "packageVersion": version}
    if data.get("query") != expected_query:
        raise ValueError("Marketplace did not render the requested release")
    queries = data["props"]["pageProps"]["dehydratedState"]["queries"]

    def lookup(key):
        for query in queries:
            if query["queryKey"] == key:
                return query["state"]["data"]
        raise ValueError(f"Missing public Marketplace data: {key[0]}")

    metadata = lookup(["packageMetadata", ACCOUNT, PACKAGE, version])
    versions = lookup(["packageVersions", ACCOUNT, PACKAGE])
    if metadata["public"] is not True or metadata["repoKey"] != f"{ACCOUNT}/{PACKAGE}" or metadata["type"] != "provider":
        raise ValueError("Release is not a public provider listing")
    if metadata["digest"] != digest:
        raise ValueError("Marketplace has not indexed the package with its updated assets")
    if not any(item["subscription_version"] == version for item in versions["versions"]):
        raise ValueError("Release is missing from public versions")
    if svg_shape(icon) != svg_shape(expected_icon):
        raise ValueError("Marketplace is not displaying the official Archestra SVG logo")
    return metadata


def download(url):
    with urlopen(url, timeout=30) as response:
        content = response.read(5 * 1024 * 1024 + 1)
    if len(content) > 5 * 1024 * 1024:
        raise ValueError("Unexpectedly large Marketplace response")
    return content


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version")
    parser.add_argument("--digest", required=True)
    parser.add_argument("--timeout", type=int, default=600)
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+", args.version):
        parser.error("Expected a stable semantic release tag")
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", args.digest):
        parser.error("Expected an OCI sha256 digest")
    url = f"{ORIGIN}/providers/{ACCOUNT}/{PACKAGE}/{args.version}"
    icon_url = f"{ORIGIN}/api/packageIcon/{ACCOUNT}/{PACKAGE}/{args.version}?type=provider"
    deadline = time.monotonic() + args.timeout
    while True:
        try:
            metadata = verify(download(url), download(icon_url), args.version, args.digest, ICON.read_bytes())
            print(f"Verified {url}: {metadata['digest']}, public listing and official logo", flush=True)
            print(f"Marketplace current version: {metadata.get('currentVersion')}", flush=True)
            return
        except (OSError, ValueError, KeyError, TypeError, ET.ParseError) as error:
            if time.monotonic() >= deadline:
                raise SystemExit(f"Public Marketplace verification failed: {error}") from error
            print(f"Waiting for Marketplace indexing: {error}", flush=True)
            time.sleep(min(15, max(0, deadline - time.monotonic())))


if __name__ == "__main__":
    main()
