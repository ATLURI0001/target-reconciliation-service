#!/usr/bin/env bash
# Deletes only resources created by lab-up, including its disposable database.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! -d .lab ]]; then echo 'No project lab state to clean.'; exit 0; fi
[[ -f .lab/owned.flag && "$(cat .lab/owned.flag)" == "$PWD" ]] || { echo 'Unrecognized lab ownership marker; refusing cleanup.' >&2; exit 1; }
export TRS_EVIDENCE_DIR=${TRS_EVIDENCE_DIR:-evidence/lab-local}
mkdir -p "$TRS_EVIDENCE_DIR"
status=0
if [[ -f .lab/api.pid ]]; then
  pid=$(cat .lab/api.pid)
  [[ "$pid" =~ ^[0-9]+$ ]] || { echo 'Invalid recorded process id; refusing to send signals.' >&2; exit 1; }
  command=$(ps -ww -p "$pid" -o args= 2>/dev/null || true)
  if [[ "$command" == "$PWD/bin/target-reconciliation-service" ]]; then
    kill -TERM "$pid" 2>/dev/null || true
    for ((attempt=0;attempt<12;attempt++)); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    # An exited child may remain as a zombie until its parent reaps it.
    process_state=$(ps -p "$pid" -o stat= 2>/dev/null || true)
    if [[ -n "$process_state" && "$process_state" != Z* ]]; then echo 'Controller did not stop after SIGTERM; inspect before cleanup.' >&2; status=1; fi
  elif [[ -n "$command" ]]; then
    echo 'Recorded PID now belongs to another command; no signal was sent.' >&2
  fi
fi
if [[ -f .lab/kubeconfig ]]; then
  KUBECONFIG="$PWD/.lab/kubeconfig" kubectl --context kind-target-reconciliation-service-lab -n trs-demo get deployment,pods -o json > "$TRS_EVIDENCE_DIR/final-kubernetes-state.json" 2>/dev/null || true
fi
if [[ -f .lab/cluster-created.flag ]]; then
  if ! kind delete cluster --name target-reconciliation-service-lab; then status=1; fi
fi
if [[ -f .lab/runtime.env && -f .lab/database-created.flag ]]; then
  if ! docker compose --project-name trs-lab --env-file .lab/runtime.env down --volumes; then status=1; fi
fi
# Remove credentials even when an external tool's teardown reports an error.
rm -f .lab/runtime.env .lab/kubeconfig
if ((status == 0)); then rm -rf .lab; else echo 'Cleanup was incomplete; inspect .lab ownership/PID records. Temporary credentials were removed.' >&2; fi
exit "$status"
