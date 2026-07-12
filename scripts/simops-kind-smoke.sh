#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run the Kubernetes SimOps adapter through a local Kind cluster on OrbStack.

Usage:
  scripts/simops-kind-smoke.sh [--timeout seconds] [--build auto|always|never]

The smoke creates an isolated Kind cluster, loads the gateway and worker images,
launches workers through POST /api/simops/runs, verifies gateway-only inputs,
ingest, lifecycle sync, TTL policy, failed-Job retention, and forced cleanup.
USAGE
}

TIMEOUT="${SIMOPS_KIND_TIMEOUT:-240}"
BUILD_MODE="${SIMOPS_KIND_BUILD:-auto}"
CLUSTER_NAME="${SIMOPS_KIND_CLUSTER:-radiant-simops-smoke}"
NAMESPACE="${SIMOPS_KIND_NAMESPACE:-radiant-simops}"
GATEWAY_IMAGE="${SIMOPS_KIND_GATEWAY_IMAGE:-radiant-slurm-gateway:kind}"
WORKER_IMAGE="${SIMOPS_KIND_WORKER_IMAGE:-radiant-simops-generator:kind}"
FORCE_CLEANUP="${SIMOPS_KIND_FORCE_CLEANUP:-1}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --timeout) TIMEOUT="${2:?--timeout requires seconds}"; shift 2 ;;
    --build) BUILD_MODE="${2:?--build requires auto, always, or never}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT" -le 0 ]]; then
  echo "--timeout must be a positive integer." >&2
  exit 2
fi
case "$BUILD_MODE" in auto|always|never) ;; *) echo "--build must be auto, always, or never." >&2; exit 2 ;; esac

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

export PATH="${SIMOPS_KIND_BIN_DIR:-/tmp/radiant-tools/bin}:/opt/homebrew/bin:/usr/local/bin:$PATH"
export DOCKER_CONTEXT="${SIMOPS_DOCKER_CONTEXT:-orbstack}"
export KUBECONFIG="${SIMOPS_KIND_KUBECONFIG:-/tmp/${CLUSTER_NAME}.kubeconfig}"

manifest_file="$(mktemp /tmp/radiant-kind-manifest.XXXXXX.yaml)"
port_forward_log="$(mktemp /tmp/radiant-kind-port-forward.XXXXXX.log)"
port_forward_pid=""
created_runs=()

run() {
  printf '+ '; printf '%q ' "$@"; printf '\n'
  "$@"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || { echo "$1 is required." >&2; exit 127; }
}

cleanup() {
  if [[ -n "$port_forward_pid" ]]; then kill "$port_forward_pid" >/dev/null 2>&1 || true; fi
  if [[ "$FORCE_CLEANUP" == "1" ]]; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
    rm -f "$KUBECONFIG"
  fi
  rm -f "$manifest_file" "$port_forward_log"
}
trap cleanup EXIT

require_command docker
require_command kind
require_command kubectl
require_command curl
require_command node

docker image inspect "$GATEWAY_IMAGE" >/dev/null 2>&1 && gateway_exists=1 || gateway_exists=0
docker image inspect "$WORKER_IMAGE" >/dev/null 2>&1 && worker_exists=1 || worker_exists=0
if [[ "$BUILD_MODE" == "always" || ( "$BUILD_MODE" == "auto" && "$gateway_exists" == "0" ) ]]; then
  run docker --context "$DOCKER_CONTEXT" build --target gateway-runtime -f deploy/slurm-gateway.Dockerfile -t "$GATEWAY_IMAGE" .
elif [[ "$BUILD_MODE" == "never" && "$gateway_exists" == "0" ]]; then
  echo "Missing gateway image $GATEWAY_IMAGE with --build never." >&2; exit 1
fi
if [[ "$BUILD_MODE" == "always" || ( "$BUILD_MODE" == "auto" && "$worker_exists" == "0" ) ]]; then
  run docker --context "$DOCKER_CONTEXT" build -f deploy/simops-generator.Dockerfile -t "$WORKER_IMAGE" .
elif [[ "$BUILD_MODE" == "never" && "$worker_exists" == "0" ]]; then
  echo "Missing worker image $WORKER_IMAGE with --build never." >&2; exit 1
fi

rm -f "$KUBECONFIG"
run kind create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG" --wait "${TIMEOUT}s"
run kind load docker-image --name "$CLUSTER_NAME" "$GATEWAY_IMAGE" "$WORKER_IMAGE"

cat >"$manifest_file" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: simops-gateway
  namespace: ${NAMESPACE}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: simops-worker
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: simops-runtime
  namespace: ${NAMESPACE}
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "delete"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: simops-runtime
  namespace: ${NAMESPACE}
subjects:
  - kind: ServiceAccount
    name: simops-gateway
    namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: simops-runtime
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: slurm-gateway
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels: {app: slurm-gateway}
  template:
    metadata:
      labels: {app: slurm-gateway}
    spec:
      serviceAccountName: simops-gateway
      containers:
        - name: gateway
          image: ${GATEWAY_IMAGE}
          imagePullPolicy: Never
          ports: [{name: http, containerPort: 8080}]
          env:
            - {name: SLURM_GATEWAY_REQUIRE_CLIENT_CERT, value: "false"}
            - {name: SIMOPS_WORKER_RUNTIME, value: "kubernetes"}
            - {name: SIMOPS_WORKER_IMAGE, value: "${WORKER_IMAGE}"}
            - {name: SIMOPS_WORKER_MANIFEST_ROOT, value: "/examples/simulation-ops"}
            - {name: SIMOPS_WORKER_INGEST_BASE_URL, value: "http://slurm-gateway.${NAMESPACE}.svc.cluster.local:8080"}
            - {name: SIMOPS_WORKER_KUBERNETES_NAMESPACE, value: "${NAMESPACE}"}
            - {name: SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT, value: "simops-worker"}
            - {name: SIMOPS_WORKER_CLEANUP_TTL, value: "60s"}
            - {name: SIMOPS_WORKER_FRAME_OVERRIDE, value: "2"}
          readinessProbe:
            httpGet: {path: /readyz, port: http}
            periodSeconds: 2
---
apiVersion: v1
kind: Service
metadata:
  name: slurm-gateway
  namespace: ${NAMESPACE}
spec:
  selector: {app: slurm-gateway}
  ports: [{name: http, port: 8080, targetPort: http}]
EOF

run kubectl apply -f "$manifest_file"
run kubectl -n "$NAMESPACE" rollout status deployment/slurm-gateway --timeout="${TIMEOUT}s"

start_port_forward() {
  if [[ -n "$port_forward_pid" ]]; then kill "$port_forward_pid" >/dev/null 2>&1 || true; wait "$port_forward_pid" >/dev/null 2>&1 || true; fi
  : >"$port_forward_log"
  kubectl -n "$NAMESPACE" port-forward service/slurm-gateway 18081:8080 >"$port_forward_log" 2>&1 &
  port_forward_pid="$!"
  deadline=$((SECONDS + TIMEOUT))
  until curl -fsS http://127.0.0.1:18081/readyz >/dev/null 2>&1; do
    [[ "$SECONDS" -lt "$deadline" ]] || { cat "$port_forward_log" >&2; return 1; }
    sleep 1
  done
}

start_port_forward

create_run() {
  local scenario="$1" suffix="$2" response run_id
  response="$(curl -fsS -X POST http://127.0.0.1:18081/api/simops/runs -H 'Content-Type: application/json' --data "{\"scenario_id\":\"${scenario}\",\"worker_kinds\":[\"scheduler\"],\"launch_mode\":\"auto\",\"idempotency_key\":\"kind-${suffix}-$(date +%s)\"}")"
  run_id="$(node scripts/simops-smoke-json.mjs run-id <<<"$response")"
  created_runs+=("$run_id")
  printf '%s' "$run_id"
}

wait_for_state() {
  local run_id="$1" state="$2" require_frames="${3:-}" status
  while [[ "$SECONDS" -lt "$deadline" ]]; do
    status="$(curl -fsS "http://127.0.0.1:18081/api/simops/runs/${run_id}")"
    if node scripts/simops-smoke-json.mjs runtime-worker "$state" $require_frames --runtime kubernetes <<<"$status"; then echo; return 0; fi
    sleep 2
  done
  echo "Timed out waiting for ${state} on ${run_id}." >&2; return 1
}

success_run="$(create_run scheduler-drift success)"
success_job="$(kubectl -n "$NAMESPACE" get jobs -l "simops.run_id=${success_run}" -o jsonpath='{.items[0].metadata.name}')"
[[ -n "$success_job" ]] || { echo "No Job created for ${success_run}." >&2; exit 1; }

for label in simops.run_id simops.worker_id simops.worker_kind; do
  kubectl -n "$NAMESPACE" get job "$success_job" -o "jsonpath={.metadata.labels.${label//./\\.}}" | grep -q .
done
ttl="$(kubectl -n "$NAMESPACE" get job "$success_job" -o jsonpath='{.spec.ttlSecondsAfterFinished}')"
[[ "$ttl" == "60" ]] || { echo "Expected ttlSecondsAfterFinished=60, got ${ttl}." >&2; exit 1; }

pod_name="$(kubectl -n "$NAMESPACE" get pods -l "simops.run_id=${success_run}" -o jsonpath='{.items[0].metadata.name}')"
pod_json="$(kubectl -n "$NAMESPACE" get pod "$pod_name" -o json)"
node -e '
const pod=JSON.parse(process.argv[1]); const c=pod.spec.containers[0]; const text=[...(c.args||[]),...(c.env||[]).flatMap(e=>[e.name,e.value||""])].join(" ");
for (const required of ["/internal/simops/runs/","/ingest","--ingest-token","--result-ingest-token"]) if (!text.includes(required)) process.exit(1);
for (const forbidden of ["REDPANDA","POSTGRES","ICEBERG","AWS_"]) if (text.includes(forbidden)) process.exit(1);
' "$pod_json"

wait_for_state "$success_run" succeeded --frames
echo "cluster_context=$(kubectl config current-context) namespace=${NAMESPACE} job_name=${success_job} run_id=${success_run} final_lifecycle=succeeded"
run kubectl -n "$NAMESPACE" delete job "$success_job" --wait=true

run kubectl -n "$NAMESPACE" set env deployment/slurm-gateway SIMOPS_WORKER_IMAGE=registry.invalid/radiant/missing:issue-25
run kubectl -n "$NAMESPACE" rollout status deployment/slurm-gateway --timeout="${TIMEOUT}s"
start_port_forward
failure_run="$(create_run scheduler-drift failure)"
failure_job="$(kubectl -n "$NAMESPACE" get jobs -l "simops.run_id=${failure_run}" -o jsonpath='{.items[0].metadata.name}')"
wait_for_state "$failure_run" image-pull-failed
kubectl -n "$NAMESPACE" get job "$failure_job" >/dev/null
echo "cluster_context=$(kubectl config current-context) namespace=${NAMESPACE} job_name=${failure_job} run_id=${failure_run} final_lifecycle=image-pull-failed retained=true"
run kubectl -n "$NAMESPACE" delete job "$failure_job" --wait=true

echo "Kind/client-go SimOps smoke passed."
