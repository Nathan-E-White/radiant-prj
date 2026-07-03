#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Configure a local Vault PKI role for Slurm gateway client certificates.

Required environment:
  VAULT_ADDR
  VAULT_TOKEN

Optional environment:
  VAULT_PKI_PATH        default: pki
  VAULT_ROLE           default: slurm-gateway-client
  VAULT_ALLOWED_DOMAIN default: cluster.local
  VAULT_MAX_TTL        default: 24h
  VAULT_OUTPUT_DIR     default: .local/vault

This helper is for local development only. It does not write Vault tokens or
private keys into source-controlled paths.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

: "${VAULT_ADDR:?VAULT_ADDR is required}"
: "${VAULT_TOKEN:?VAULT_TOKEN is required}"

repo_root="$(git rev-parse --show-toplevel)"
pki_path="${VAULT_PKI_PATH:-pki}"
role="${VAULT_ROLE:-slurm-gateway-client}"
allowed_domain="${VAULT_ALLOWED_DOMAIN:-cluster.local}"
max_ttl="${VAULT_MAX_TTL:-24h}"
output_dir="${VAULT_OUTPUT_DIR:-${repo_root}/.local/vault}"

mkdir -p "$output_dir"

if ! vault secrets list -format=json | grep -q "\"${pki_path}/\""; then
  vault secrets enable -path="$pki_path" pki
fi

vault secrets tune -max-lease-ttl=8760h "$pki_path"

vault write -field=certificate "${pki_path}/root/generate/internal" \
  common_name="Radiant local Slurm gateway CA" \
  ttl=8760h > "${output_dir}/vault_ca.crt"

vault write "${pki_path}/config/urls" \
  issuing_certificates="${VAULT_ADDR}/v1/${pki_path}/ca" \
  crl_distribution_points="${VAULT_ADDR}/v1/${pki_path}/crl"

vault write "${pki_path}/roles/${role}" \
  allowed_domains="$allowed_domain" \
  allow_subdomains=false \
  allow_any_name=false \
  enforce_hostnames=false \
  client_flag=true \
  server_flag=false \
  max_ttl="$max_ttl"

echo "Vault PKI role '${role}' configured at '${pki_path}'. CA written to ${output_dir}/vault_ca.crt."
