#!/usr/bin/env bash
# Install the pinned controller used by the provider acceptance environment.
# KUBECONFIG must already identify the caller's isolated test cluster.
set -euo pipefail
: "${KUBECONFIG:?KUBECONFIG must identify an isolated test cluster}"
manifest=$(mktemp)
trap 'rm -f "$manifest"' EXIT
curl --retry 3 --retry-all-errors --connect-timeout 10 --max-time 120 -fsSL \
  https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v1.0.1/sandbox-with-extensions.yaml \
  --output "$manifest"
python3 - "$manifest" <<'CHECKSUM'
from pathlib import Path
import hashlib, sys
expected = '460c1e0272c793c98de82482aed57db1bddb0afcb979c19325dc6cc52289c44c'
if hashlib.sha256(Path(sys.argv[1]).read_bytes()).hexdigest() != expected:
    raise SystemExit('Agent Sandbox manifest checksum mismatch')
CHECKSUM
kubectl apply --server-side -f "$manifest"
kubectl rollout status deployment/agent-sandbox-controller -n agent-sandbox-system --timeout=120s
