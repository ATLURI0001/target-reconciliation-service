#!/usr/bin/env bash
# An explicitly requested test-only failed-rollout demonstration.
set -euo pipefail
cd "$(dirname "$0")/.."
for tool in kubectl python3 curl; do command -v "$tool" >/dev/null || { echo "Install $tool before running the lab." >&2; exit 1; }; done
[[ "$(kubectl config current-context)" == kind-resilienceplane-lab ]] || { echo 'Refusing mutations: expected dedicated context kind-resilienceplane-lab.' >&2; exit 1; }
[[ "${RP_DEMO_EXECUTE:-}" == yes ]] || { echo 'This test changes the rp-demo/catalog image, disables dry-run, and approves its rollback.'; echo 'After starting the controller, explicitly run RP_DEMO_EXECUTE=yes ./scripts/demo.sh'; exit 1; }
[[ -f .env ]] || { echo 'Run make init first.' >&2; exit 1; }
set -a
. ./.env
set +a
api=${RP_API_URL:-http://127.0.0.1:8080}
[[ "$api" == http://127.0.0.1:8080 ]] || { echo 'Initial demonstration is restricted to loopback API8080.' >&2; exit 1; }
auth=(-H "Authorization: Bearer $RP_OPERATOR_TOKEN" -H 'Content-Type: application/json')
# The supplied deployment must first actually become healthy; its running digest
# supplies a locally verified recovery image, rather than a fabricated digest.
kubectl -n rp-demo rollout status deployment/catalog --timeout=120s
image=$(kubectl -n rp-demo get pods -l app=catalog -o 'jsonpath={.items[0].status.containerStatuses[0].imageID}')
image=${image#docker-pullable://}
[[ "$image" =~ @sha256:[a-f0-9]{64}$ ]] || { echo 'Runtime did not expose a pullable digest; choose a registry digest manually.' >&2; exit 1; }
export RP_DEMO_IMAGE="$image"
python3 - <<'PY' | kubectl apply -f -
import json,os
print(json.dumps({'apiVersion':'reliability.resilienceplane.io/v1alpha1','kind':'ReliabilityTarget','metadata':{'name':'catalog','namespace':'rp-demo'},'spec':{'deployment':'catalog','container':'catalog','healthyImage':os.environ['RP_DEMO_IMAGE']}}))
PY
# Explicitly replace policy only inside this disposable test lab.
curl -fsS "${auth[@]}" -X PUT "$api/v1/policy" --data '{"namespaces":["rp-demo"],"maxReplicas":3,"maxConcurrent":1,"freshnessSeconds":30,"detectionSeconds":10,"dryRun":false,"emergencyStop":false}' >/dev/null
# Guaranteed nonexistent image causes a genuine failed rollout; no prod namespace.
kubectl -n rp-demo set image deployment/catalog catalog=nginx:resilienceplane-deliberate-invalid-release
incident=''
for ((attempt=0;attempt<40;attempt++)); do
 snapshot=$(curl -fsS "${auth[@]}" "$api/v1/snapshot")
 incident=$(printf '%s' "$snapshot" | python3 -c 'import sys,json;s=json.load(sys.stdin);xs=[i for i in s["incidents"].values() if i["target"]=="local/rp-demo/catalog" and i["state"]=="AwaitingApproval"]; print(max(xs,key=lambda i:i["created"])["id"] if xs else "")')
 [[ -n "$incident" ]] && break
 sleep 2
done
[[ -n "$incident" ]] || { echo 'No eligible incident; inspect controller logs, policy, and audit.' >&2; exit 1; }
# Verify a known viewer cannot authorize; rejection must not mutate the workload.
code=$(curl -sS -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $RP_VIEWER_TOKEN" -X POST "$api/v1/incidents/$incident/approve")
[[ "$code" == 403 ]] || { echo "Expected policy denial, got HTTP $code" >&2; exit 1; }
curl -fsS "${auth[@]}" -X POST "$api/v1/incidents/$incident/approve" >/dev/null
for ((attempt=0;attempt<60;attempt++)); do
 state=$(curl -fsS "${auth[@]}" "$api/v1/snapshot" | python3 -c 'import sys,json;s=json.load(sys.stdin);print(s["incidents"][sys.argv[1]]["state"])' "$incident")
 if [[ "$state" == Resolved ]]; then echo "Verified recovery: incident $incident; inspect /v1/audit for identity-linked timeline."; exit 0; fi
 [[ "$state" != Failed && "$state" != Cancelled ]] || { echo "Recovery stopped: $state" >&2; exit 1; }
 sleep 2
done
echo 'Verification deadline exceeded. Inspect the deployment; no success is inferred.' >&2
exit 1
