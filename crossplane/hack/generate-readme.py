#!/usr/bin/env python3
"""Embed the user-facing README in package metadata for Upbound Marketplace."""

from pathlib import Path

root = Path(__file__).resolve().parents[1]
readme = (root / "README.md").read_text().split("\n## Development\n", 1)[0].rstrip()
package = root / "package/crossplane.yaml"
header, rest = package.read_text().split("    meta.crossplane.io/readme: |\n", 1)
_, spec = rest.split("\nspec:", 1)
block = "\n".join("      " + line if line else "" for line in readme.splitlines())
package.write_text(header + "    meta.crossplane.io/readme: |\n" + block + "\nspec:" + spec)
