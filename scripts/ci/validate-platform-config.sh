#!/usr/bin/env bash
# Renders all payflow-platform-config overlays (Unit 7 verification; kubeconform optional in Unit 8).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CFG="${ROOT}/payflow-platform-config"

if command -v kubectl >/dev/null 2>&1; then
  KUSTOMIZE=(kubectl kustomize)
elif command -v kustomize >/dev/null 2>&1; then
  KUSTOMIZE=(kustomize build)
else
  echo "error: need kubectl or kustomize on PATH" >&2
  exit 1
fi

for env in dev staging prod; do
  echo "== ${KUSTOMIZE[*]} overlays/${env} =="
  "${KUSTOMIZE[@]}" "${CFG}/overlays/${env}" >/dev/null
done

echo "== ${KUSTOMIZE[*]} observability =="
"${KUSTOMIZE[@]}" "${CFG}/observability" >/dev/null

if command -v kubeconform >/dev/null 2>&1; then
  for env in dev staging prod; do
    echo "== kubeconform overlays/${env} =="
    "${KUSTOMIZE[@]}" "${CFG}/overlays/${env}" | kubeconform -strict -summary -
  done
else
  echo "(kubeconform not installed; skipping schema validation)"
fi

echo "ok: all overlays render"
