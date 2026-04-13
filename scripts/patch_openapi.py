"""Patch OpenAPI 3.0 spec to fix numeric exclusiveMinimum/exclusiveMaximum values.

Some backends emit numeric exclusiveMinimum/exclusiveMaximum (an OpenAPI 3.1 / JSON
Schema 2020-12 feature) in an otherwise 3.0 spec. This script converts them to the
3.0-compatible boolean form:
  exclusiveMinimum: 0  ->  minimum: 0, exclusiveMinimum: true
  exclusiveMaximum: 1  ->  maximum: 1, exclusiveMaximum: true

Boolean values (already 3.0-compatible) are left unchanged.

Usage: cat spec.json | python3 scripts/patch_openapi.py > patched.json
"""

import json
import sys


def patch(obj):
    if isinstance(obj, dict):
        if "exclusiveMinimum" in obj:
            if isinstance(obj["exclusiveMinimum"], (int, float)) and not isinstance(obj["exclusiveMinimum"], bool):
                val = obj.pop("exclusiveMinimum")
                obj["minimum"] = val
                obj["exclusiveMinimum"] = True
        if "exclusiveMaximum" in obj:
            if isinstance(obj["exclusiveMaximum"], (int, float)) and not isinstance(obj["exclusiveMaximum"], bool):
                val = obj.pop("exclusiveMaximum")
                obj["maximum"] = val
                obj["exclusiveMaximum"] = True
        for key in list(obj.keys()):
            patch(obj[key])
    elif isinstance(obj, list):
        for item in obj:
            patch(item)


spec = json.load(sys.stdin)
patch(spec)
json.dump(spec, sys.stdout)
