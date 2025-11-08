#!/usr/bin/env bash
# Parse YAML under payflow-platform-config (syntax only).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CFG="${ROOT}/payflow-platform-config"
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 required for lint-yaml.sh" >&2
  exit 1
fi
export CFG
python3 <<'PY'
import os, pathlib, sys, yaml
root = pathlib.Path(os.environ["CFG"])
errs = 0
paths = sorted({*root.rglob("*.yaml"), *root.rglob("*.yml")})
for path in paths:
    try:
        text = path.read_text()
    except OSError as e:
        print(e, file=sys.stderr)
        errs += 1
        continue
    try:
        for _ in yaml.safe_load_all(text):
            pass
    except yaml.YAMLError as e:
        print(f"{path}: {e}", file=sys.stderr)
        errs += 1
sys.exit(errs)
PY
