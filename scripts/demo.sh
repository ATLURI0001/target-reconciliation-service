#!/usr/bin/env bash
# A real, explicitly requested failed-rollout demonstration in a disposable lab.
set -euo pipefail
cd "$(dirname "$0")/.."
for tool in kubectl python3 curl; do command -v "$tool" >/dev/null || { echo "Install $tool before running the lab." >&2; exit 1; }; done
[[ "$(kubectl config current-context)" == kind-target-reconciliation-service-lab ]] || { echo 'Refusing mutations: expected dedicated context kind-target-reconciliation-service-lab.' >&2; exit 1; }
[[ "${TRS_DEMO_EXECUTE:-}" == yes ]] || { echo 'This test changes trs-demo/catalog, temporarily disables dry-run, and approves its rollback.'; echo 'Explicitly run make lab-demo in the disposable lab, or TRS_DEMO_EXECUTE=yes ./scripts/demo.sh.'; exit 1; }
credential_file=${TRS_ENV_FILE:-.env}
[[ -f "$credential_file" ]] || { echo 'Local lab credentials not found; run make lab-up or make init first.' >&2; exit 1; }
set -a
. "$credential_file"
set +a
api=${TRS_API_URL:-http://127.0.0.1:8080}
[[ "$api" == http://127.0.0.1:8080 ]] || { echo 'The initial demonstration is restricted to loopback API8080.' >&2; exit 1; }
evidence=${TRS_EVIDENCE_DIR:-evidence/lab-local}
mkdir -p "$evidence"
auth=(-H "Authorization: Bearer $TRS_OPERATOR_TOKEN" -H 'Content-Type: application/json')
original_policy=$(curl -fsS "${auth[@]}" "$api/v1/snapshot" | python3 -c 'import json,sys;print(json.dumps(json.load(sys.stdin)["policy"]))')
forward_pid=''
cleanup() {
  status=$?
  if [[ -n "$forward_pid" ]]; then kill "$forward_pid" 2>/dev/null || true; wait "$forward_pid" 2>/dev/null || true; fi
  if ! curl -fsS "${auth[@]}" -X PUT "$api/v1/policy" --data "$original_policy" >/dev/null; then
    echo 'Could not restore the original policy; inspect the lab before further use.' >&2
    status=1
  fi
  exit "$status"
}
trap cleanup EXIT
wait_target_ready() {
  local desired="$1" stage="$2"
  for ((attempt=0;attempt<90;attempt++)); do
    target=$(kubectl -n trs-demo get reliabilitytarget catalog -o json)
    if printf '%s' "$target" | python3 -c 'import json,sys;t=json.load(sys.stdin);g=t["metadata"]["generation"];cs=t.get("status",{}).get("conditions",[]);sys.exit(0 if any(c["type"]=="Ready" and c["status"]==sys.argv[1] and c.get("observedGeneration")==g for c in cs) else 1)' "$desired"; then
      printf '%s\n' "$target" > "$evidence/target-${stage}.json"
      return 0
    fi
    sleep 2
  done
  echo "Target Ready=$desired was not observed at the current CR generation." >&2
  return 1
}
probe_service() {
  local stage="$1"
  python3 - <<'PYPORT'
import socket
with socket.socket() as s:
    try:
        s.bind(('127.0.0.1', 18080))
    except OSError:
        raise SystemExit('Loopback port 18080 is occupied; refusing to probe an unrelated server.')
PYPORT
  kubectl -n trs-demo port-forward --address=127.0.0.1 service/catalog 18080:8080 > "$evidence/service-forward-${stage}.log" 2>&1 &
  forward_pid=$!
  for ((probe=0;probe<30;probe++)); do
    kill -0 "$forward_pid" 2>/dev/null || { echo 'Lab HTTP port-forward failed; inspect its log.' >&2; return 1; }
    if [[ "$(cat "$evidence/service-forward-${stage}.log")" == *"Forwarding from 127.0.0.1:18080"* ]] && curl --fail --silent --max-time 2 http://127.0.0.1:18080/ > "$evidence/service-${stage}.html"; then
      kill "$forward_pid" 2>/dev/null || true
      wait "$forward_pid" 2>/dev/null || true
      forward_pid=''
      return 0
    fi
    sleep 1
  done
  echo "No HTTP success from the actual catalog service ($stage)." >&2
  return 1
}
# Derive the known-good digest from a genuinely ready running pod.
kubectl -n trs-demo rollout status deployment/catalog --timeout=180s
image=$(kubectl -n trs-demo get pods -l app=trs-catalog -o 'jsonpath={.items[0].status.containerStatuses[0].imageID}')
image=${image#docker-pullable://}
[[ "$image" =~ @sha256:[a-f0-9]{64}$ ]] || { echo 'Runtime did not expose a pullable digest; no recovery image is inferred.' >&2; exit 1; }
export TRS_DEMO_IMAGE="$image"
python3 - <<'PY' | kubectl apply -f -
import json,os
print(json.dumps({'apiVersion':'reliability.trs.io/v1alpha1','kind':'ReliabilityTarget','metadata':{'name':'catalog','namespace':'trs-demo'},'spec':{'deployment':'catalog','container':'catalog','healthyImage':os.environ['TRS_DEMO_IMAGE']}}))
PY
wait_target_ready True baseline
probe_service before
curl -fsS "${auth[@]}" "$api/v1/snapshot" > "$evidence/baseline-snapshot.json"
curl -fsS "${auth[@]}" -X PUT "$api/v1/policy" --data '{"namespaces":["trs-demo"],"maxReplicas":3,"maxConcurrent":1,"freshnessSeconds":30,"detectionSeconds":10,"dryRun":false,"emergencyStop":false}' >/dev/null
bad_image=nginx:trs-deliberate-invalid-release
kubectl -n trs-demo set image deployment/catalog "catalog=$bad_image"
wait_target_ready False failed
incident=''
for ((attempt=0;attempt<60;attempt++)); do
  snapshot=$(curl -fsS "${auth[@]}" "$api/v1/snapshot")
  incident=$(printf '%s' "$snapshot" | python3 -c 'import sys,json;s=json.load(sys.stdin);xs=[i for i in s["incidents"].values() if i["target"]=="local/trs-demo/catalog" and i["state"]=="AwaitingApproval" and i["plan"]["target"]["image"]==sys.argv[1]];print(max(xs,key=lambda i:i["created"])["id"] if xs else "")' "$bad_image")
  [[ -n "$incident" ]] && break
  sleep 2
done
[[ -n "$incident" ]] || { echo 'No eligible incident; inspect the controller, policy, and audit evidence.' >&2; exit 1; }
curl -fsS "${auth[@]}" "$api/v1/snapshot" > "$evidence/awaiting-approval-snapshot.json"
before_denial=$(kubectl -n trs-demo get deployment catalog -o 'jsonpath={.spec.template.spec.containers[0].image}')
code=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TRS_VIEWER_TOKEN" -X POST "$api/v1/incidents/$incident/approve")
after_denial=$(kubectl -n trs-demo get deployment catalog -o 'jsonpath={.spec.template.spec.containers[0].image}')
[[ "$code" == 403 && "$before_denial" == "$bad_image" && "$after_denial" == "$before_denial" ]] || { echo 'Viewer denial did not preserve the expected Deployment image.' >&2; exit 1; }
printf '%s\n' 'Viewer approval: HTTP403; Deployment image unchanged.' > "$evidence/security-enforcement.txt"
curl -fsS "${auth[@]}" -X POST "$api/v1/incidents/$incident/approve" >/dev/null
for ((attempt=0;attempt<90;attempt++)); do
  state=$(curl -fsS "${auth[@]}" "$api/v1/snapshot" | python3 -c 'import sys,json;s=json.load(sys.stdin);print(s["incidents"][sys.argv[1]]["state"])' "$incident")
  if [[ "$state" == Resolved ]]; then
    wait_target_ready True recovered
    kubectl -n trs-demo rollout status deployment/catalog --timeout=120s
    actual=$(kubectl -n trs-demo get deployment catalog -o 'jsonpath={.spec.template.spec.containers[0].image}')
    [[ "$actual" == "$image" ]] || { echo 'Resolved incident did not restore the recorded digest.' >&2; exit 1; }
    probe_service after
    curl -fsS "${auth[@]}" "$api/v1/snapshot" > "$evidence/resolved-snapshot.json"
    curl -fsS "${auth[@]}" "$api/v1/audit?limit=200" > "$evidence/audit.json"
    curl -fsS "$api/metrics" > "$evidence/metrics.txt"
    kubectl -n trs-demo get deployment catalog -o json > "$evidence/recovered-deployment.json"
    echo "Verified actual service HTTP, failed rollout, viewer denial, approved rollback, and healthy digest restoration: incident $incident."
    exit 0
  fi
  [[ "$state" != Failed && "$state" != Cancelled ]] || { echo "Recovery stopped: $state" >&2; exit 1; }
  sleep 2
done
echo 'Verification deadline exceeded. No success is inferred.' >&2
exit 1
