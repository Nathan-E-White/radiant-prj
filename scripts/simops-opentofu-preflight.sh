#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
module_dir="$repo_root/infra/opentofu/simops-kind-substrate"
data_dir="${SIMOPS_TOFU_DATA_DIR:-/tmp/radiant-tofu-simops-data}"
plugin_cache="${SIMOPS_TOFU_PLUGIN_CACHE:-/tmp/radiant-tofu-plugin-cache}"
kubeconfig="$(mktemp /tmp/radiant-tofu-kubeconfig.XXXXXX)"
plan_file="$(mktemp /tmp/radiant-tofu-plan.XXXXXX)"
plan_output="$(mktemp /tmp/radiant-tofu-plan-output.XXXXXX)"

cleanup() {
  rm -f "$kubeconfig" "$plan_file" "$plan_output"
  rm -rf "$data_dir"
}
trap cleanup EXIT

mkdir -p "$data_dir" "$plugin_cache"
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
tofu init -backend=false -input=false
tofu validate
tofu plan -no-color -input=false -lock=false -refresh=false -out="$plan_file" -var="kubeconfig_path=$kubeconfig" -var="kubeconfig_context=preflight" | tee "$plan_output"

plan_summary="$(grep -E 'Plan: [0-9]+ to add, [0-9]+ to change, [0-9]+ to destroy' "$plan_output" | tail -n 1)"
[[ "$plan_summary" == "Plan: 6 to add, 0 to change, 0 to destroy." ]] || { echo "Unexpected plan summary: ${plan_summary:-missing}" >&2; exit 1; }

echo "plan_summary=${plan_summary}"
echo "namespace=radiant-simops service_account=simops-worker runtime_config_map=simops-runtime-adapter mutation=false"
