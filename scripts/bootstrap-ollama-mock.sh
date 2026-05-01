#!/usr/bin/env bash
#
# Deploys the Ollama mock service inside the current kubectl context. Used by
# CI (Kind cluster) and any local cluster running the Archestra platform via
# the same Helm chart. The backend's testProviderApiKey hits Ollama's
# /v1/models when validating LLM keys — this stub returns a static model list
# so BYOS Ollama-key tests pass without a real Ollama install.
#
# Required tools: kubectl with a context pointing at the target cluster.
#
# Idempotent: kubectl apply is safe to re-run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/k8s/ollama-mock.yaml"

echo "Applying Ollama mock manifest from ${MANIFEST}..." >&2
kubectl apply -f "$MANIFEST"

echo "Waiting for ollama-mock rollout..." >&2
kubectl rollout status deployment/ollama-mock --namespace default --timeout=60s

echo "Ollama mock ready at http://ollama-mock.default.svc.cluster.local:11434" >&2
