#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
[[ -f .lab/owned.flag && "$(cat .lab/owned.flag)" == "$PWD" && -f .lab/runtime.env && -f .lab/kubeconfig ]] || { echo 'Run make lab-up first.' >&2; exit 1; }
export KUBECONFIG="$PWD/.lab/kubeconfig"
export TRS_ENV_FILE="$PWD/.lab/runtime.env"
export TRS_EVIDENCE_DIR=${TRS_EVIDENCE_DIR:-evidence/lab-local}
# Invoking make lab-demo explicitly authorizes the documented disposable-lab action.
export TRS_DEMO_EXECUTE=yes
mkdir -p "$TRS_EVIDENCE_DIR"
./scripts/demo.sh 2>&1 | tee "$TRS_EVIDENCE_DIR/demo.log"
