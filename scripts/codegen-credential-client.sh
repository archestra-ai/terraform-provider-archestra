#!/usr/bin/env bash
# Generate newer credential operations without changing the pinned legacy client.
set -euo pipefail
mkdir -p .codegen
curl -fsS "${ARCHESTRA_BASE_URL:-http://localhost:9000}/openapi.json" -o .codegen/credentials-source.json
python3 - <<'PYTHON'
import json
from pathlib import Path
spec = json.loads(Path('.codegen/credentials-source.json').read_text())
paths = ['/api/agents/{id}/tool-exclusions', '/api/credentials', '/api/credentials/{key}',
         '/api/credentials/{key}/personal', '/api/credentials/{key}/organization']
spec['paths'] = {path: spec['paths'][path] for path in paths}
Path('.codegen/credentials.raw.json').write_text(json.dumps(spec))
PYTHON
go -C tools run ./oapi-patch -in ../.codegen/credentials.raw.json -out ../.codegen/credentials.json
go tool oapi-codegen -config oapi-credentials-config.yaml .codegen/credentials.json
go -C tools run ./client-bridge ../internal/client/archestra_client.go ../.codegen/credentials.go ../internal/client/archestra_client_manual.go
