#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Create local development certificates for the Slurm gateway under .local/certs.

Usage:
  scripts/create-local-gateway-certs.sh [--force]

Outputs:
  .local/certs/ca.crt
  .local/certs/server.crt
  .local/certs/server.key
  .local/certs/client-authorized.crt
  .local/certs/client-authorized.key
  .local/certs/client-unauthorized.crt
  .local/certs/client-unauthorized.key

These files are local runtime secrets and are ignored by git.
USAGE
}

FORCE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --force) FORCE=1 ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

repo_root="$(git rev-parse --show-toplevel)"
cert_dir="${repo_root}/.local/certs"
days_valid="${LOCAL_GATEWAY_CERT_DAYS:-30}"

mkdir -p "$cert_dir"

ca_key="${cert_dir}/ca.key"
ca_cert="${cert_dir}/ca.crt"
server_key="${cert_dir}/server.key"
server_csr="${cert_dir}/server.csr"
server_cert="${cert_dir}/server.crt"
server_conf="${cert_dir}/server.openssl.cnf"
client_conf="${cert_dir}/client.openssl.cnf"

if [[ "$FORCE" -eq 0 && -e "$server_cert" ]]; then
  echo "Local gateway certificates already exist under ${cert_dir}. Use --force to recreate them."
  exit 0
fi

cat > "$server_conf" <<'EOF'
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = localhost

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF

cat > "$client_conf" <<'EOF'
[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

openssl req -x509 -newkey rsa:4096 -sha256 -days "$days_valid" -nodes \
  -keyout "$ca_key" \
  -out "$ca_cert" \
  -subj "/CN=Radiant Local Slurm Gateway CA"

openssl req -newkey rsa:2048 -nodes \
  -keyout "$server_key" \
  -out "$server_csr" \
  -config "$server_conf"

openssl x509 -req -sha256 -days "$days_valid" \
  -in "$server_csr" \
  -CA "$ca_cert" \
  -CAkey "$ca_key" \
  -CAcreateserial \
  -out "$server_cert" \
  -extensions v3_req \
  -extfile "$server_conf"

create_client_cert() {
  local name="$1"
  local common_name="$2"
  local key="${cert_dir}/${name}.key"
  local csr="${cert_dir}/${name}.csr"
  local cert="${cert_dir}/${name}.crt"

  openssl req -newkey rsa:2048 -nodes \
    -keyout "$key" \
    -out "$csr" \
    -subj "/CN=${common_name}"

  openssl x509 -req -sha256 -days "$days_valid" \
    -in "$csr" \
    -CA "$ca_cert" \
    -CAkey "$ca_key" \
    -CAcreateserial \
    -out "$cert" \
    -extensions v3_req \
    -extfile "$client_conf"
}

create_client_cert "client-authorized" "react-backend-client"
create_client_cert "client-unauthorized" "unauthorized-client"

rm -f "$server_csr" "${cert_dir}"/*.csr "$server_conf" "$client_conf"
chmod 600 "${cert_dir}"/*.key

cat <<EOF
Local gateway certificates created under ${cert_dir}.

Example server env:
  SLURM_GATEWAY_TLS_CERT_FILE=.local/certs/server.crt
  SLURM_GATEWAY_TLS_KEY_FILE=.local/certs/server.key
  SLURM_GATEWAY_CLIENT_CA_FILE=.local/certs/ca.crt
EOF
