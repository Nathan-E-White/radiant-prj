#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
module_dir="$repo_root/infra/opentofu/simops-kind-substrate"
tmp_root="${SIMOPS_TOFU_TMP_ROOT:-/tmp}"
plugin_cache="${SIMOPS_TOFU_PLUGIN_CACHE:-/tmp/radiant-tofu-plugin-cache}"
case "$tmp_root" in /tmp|/tmp/*) ;; *) echo "SIMOPS_TOFU_TMP_ROOT must be /tmp or a child of /tmp." >&2; exit 2 ;; esac
mkdir -p "$tmp_root" "$plugin_cache"
data_dir="$(mktemp -d "$tmp_root/radiant-tofu-simops-data.XXXXXX")"
kubeconfig="$(mktemp /tmp/radiant-tofu-kubeconfig.XXXXXX)"
plan_file="$(mktemp /tmp/radiant-tofu-plan.XXXXXX)"
plan_output="$(mktemp /tmp/radiant-tofu-plan-output.XXXXXX)"

cleanup() {
  rm -f "$kubeconfig" "$plan_file" "$plan_output"
  rm -rf "$data_dir"
}
trap cleanup EXIT

cat >"$kubeconfig" <<'EOF'
apiVersion: v1
kind: Config
clusters:
  - name: preflight
    cluster:
      server: https://127.0.0.1:65535
      insecure-skip-tls-verify: true
contexts:
  - name: preflight
    context:
      cluster: preflight
      user: preflight
current-context: preflight
users:
  - name: preflight
    user:
      token: preflight-not-a-secret
EOF

export TF_DATA_DIR="$data_dir"
export TF_PLUGIN_CACHE_DIR="$plugin_cache"

cd "$module_dir"
tofu fmt -check -recursive
tofu init -backend=false -input=false -lockfile=readonly
tofu validate
tofu plan -no-color -input=false -lock=false -refresh=false -out="$plan_file" -var="kubeconfig_path=$kubeconfig" -var="kubeconfig_context=preflight" | tee "$plan_output"

plan_summary="$(grep -E 'Plan: [0-9]+ to add, [0-9]+ to change, [0-9]+ to destroy' "$plan_output" | tail -n 1)"
[[ "$plan_summary" == "Plan: 6 to add, 0 to change, 0 to destroy." ]] || { echo "Unexpected plan summary: ${plan_summary:-missing}" >&2; exit 1; }

adapter_evidence="$(tofu show -json "$plan_file" | node -e '
let raw=""; process.stdin.on("data", chunk => raw += chunk); process.stdin.on("end", () => {
  const outputs=JSON.parse(raw).planned_values.outputs;
  const env=outputs.runtime_adapter_env.value;
  const expected={SIMOPS_WORKER_RUNTIME:"kubernetes",SIMOPS_WORKER_KUBERNETES_NAMESPACE:outputs.namespace.value,SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT:outputs.worker_service_account.value,SIMOPS_WORKER_CLEANUP_TTL:"60s"};
  if (Object.keys(expected).some(key => env[key] !== expected[key]) || Object.keys(env).length !== Object.keys(expected).length) { console.error(`Adapter env mismatch: ${JSON.stringify(env)}`); process.exit(1); }
  process.stdout.write(`namespace=${outputs.namespace.value} service_account=${outputs.worker_service_account.value} runtime_config_map=${outputs.runtime_config_map.value} mutation=false`);
});
')"

echo "plan_summary=${plan_summary}"
echo "$adapter_evidence"
