#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/examples/minimal"
terraform fmt -check -recursive "$ROOT"
terraform init -backend=false -input=false
terraform validate
