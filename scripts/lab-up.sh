#!/usr/bin/env bash
# Creates a disposable, single-cluster lab, never a production context.
set -euo pipefail
cd "$(dirname "$0")/.."
for tool in docker kind kubectl python3 curl make; do
  command -v "$tool" >/dev/null || { echo "Missing $tool; run make doctor and see docs/local-lab.md." >&2; exit 1; }
done
"${GO:-go}" version >/dev/null
docker info >/dev/null
docker compose version >/dev/null
existing_containers=$(docker ps --all --filter label=com.docker.compose.project=trs-lab --quiet)
existing_volumes=$(docker volume ls --filter label=com.docker.compose.project=trs-lab --quiet)
[[ -z "$existing_containers" && -z "$existing_volumes" ]] || { echo 'Compose project trs-lab already has resources; refusing to adopt or delete them.' >&2; exit 1; }
[[ ! -e .lab ]] || { echo 'Existing .lab directory found. Inspect it and run make lab-down; refusing to adopt unknown state.' >&2; exit 1; }
clusters=$(kind get clusters)
while IFS= read -r cluster; do
  [[ "$cluster" != target-reconciliation-service-lab ]] || { echo 'The lab cluster name already exists; refusing to modify an existing cluster.' >&2; exit 1; }
done <<< "$clusters"
python3 - <<'PY'
import socket
for port in (8080, 55432, 18080):
    with socket.socket() as s:
        try:
            s.bind(('127.0.0.1', port))
        except OSError:
            raise SystemExit(f'Loopback port {port} is occupied; stop the conflicting service before lab startup.')
PY
umask 077
mkdir .lab
printf '%s\n' "$PWD" > .lab/owned.flag
export TRS_EVIDENCE_DIR=${TRS_EVIDENCE_DIR:-evidence/lab-local}
mkdir -p "$TRS_EVIDENCE_DIR"
# Only random lab credentials are used. They are never copied to evidence.
python3 - <<'PY'
import os, secrets
fd=os.open('.lab/runtime.env',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,'w') as f:
    for key in ('TRS_DATABASE_PASSWORD','TRS_OPERATOR_TOKEN','TRS_VIEWER_TOKEN','TRS_GRAFANA_PASSWORD'):
        f.write(f'{key}={secrets.token_hex(32)}\n')
    f.write('TRS_DATABASE_PORT=55432\n')
PY
trap 'status=$?; if ((status != 0)); then ./scripts/lab-down.sh || true; fi' EXIT
export KUBECONFIG="$PWD/.lab/kubeconfig"
kind create cluster --name target-reconciliation-service-lab --config deploy/kind/config.yaml --kubeconfig "$KUBECONFIG" --wait 120s
printf '%s\n' created > .lab/cluster-created.flag
kubectl --context kind-target-reconciliation-service-lab apply -f config/crd/
kubectl --context kind-target-reconciliation-service-lab wait --for=condition=Established crd/reliabilitytargets.reliability.trs.io --timeout=60s
kubectl --context kind-target-reconciliation-service-lab apply -f examples/demo/
kubectl --context kind-target-reconciliation-service-lab -n trs-demo rollout status deployment/catalog --timeout=180s
set -a
. ./.lab/runtime.env
set +a
printf '%s\n' created > .lab/database-created.flag
docker compose --project-name trs-lab --env-file .lab/runtime.env up --wait -d postgres
make build
set -a
. ./.lab/runtime.env
set +a
export DATABASE_URL="postgres://target-reconciliation-service:${TRS_DATABASE_PASSWORD}@127.0.0.1:${TRS_DATABASE_PORT}/target-reconciliation-service?sslmode=disable"
export TRS_KUBERNETES=true TRS_NAMESPACE=trs-demo TRS_CLUSTER_ID=local TRS_LISTEN=127.0.0.1:8080
nohup "$PWD/bin/target-reconciliation-service" > "$TRS_EVIDENCE_DIR/controller.log" 2>&1 < /dev/null &
printf '%s\n' "$!" > .lab/api.pid
for ((attempt=0;attempt<60;attempt++)); do
  if curl --fail --silent http://127.0.0.1:8080/readyz >/dev/null; then
    kubectl version --output=json > "$TRS_EVIDENCE_DIR/kubernetes-version.json"
    printf '%s\n' 'Disposable lab ready. Run make lab-demo, then make lab-down.'
    exit 0
  fi
  kill -0 "$(cat .lab/api.pid)" 2>/dev/null || { echo 'API process stopped; inspect the controller log.' >&2; exit 1; }
  sleep 1
done
echo 'API did not become ready before the startup deadline.' >&2
exit 1
