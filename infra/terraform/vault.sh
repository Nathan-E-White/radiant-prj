#!/usr/bin/env bash


# Enable the PKI secrets engine
vault secrets enable pki

# TODO Integrate parameters
# Tune the engine to allow certificates with up to 1 year max lifetime
vault secrets tune -max-lease-ttl=8760h pki

# TODO Integrate parameters
# Generate the self-signed Root CA inside Vault
vault write -field=certificate pki/root/generate/internal \
    common_name="My Compute Cluster Root CA" \
    ttl=8760h > vault_ca.crt

# TODO Integrate parameters
# Configure the distribution URLs that go inside issued certificates
vault write pki/config/urls \
    issuing_certificates="http://127.0.0" \
    crl_distribution_points="http://127.0.0"

# TODO Integrate parameters
# Create a strict validation Role that apps can use to request certificates
vault write pki/roles/slurm-cluster-nodes \
    allowed_domains="localhost", "cluster.local" \
    allow_subdomains=true \
    max_ttl=24h \
    allow_any_name=true
