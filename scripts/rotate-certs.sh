#!/usr/bin/env bash

# Exit immediately if any command fails
set -euo pipefail

# --- CONFIGURATION ---
TARGET_CERT="client.crt"
TARGET_KEY="client.key"
CA_CERT="ca.crt"
CA_KEY="ca.key"
THRESHOLD_DAYS=30
DAYS_VALID=365
CLIENT_SUBJ="/CN=react-backend-client"

echo "Checking expiration status for ${TARGET_CERT}..."

# 1. Verify the certificate file actually exists
if [ ! -f "$TARGET_CERT" ]; then
    echo "⚠️  ${TARGET_CERT} not found! Initiating emergency initial generation..."
    openssl req -newkey rsa:2048 -keyout "$TARGET_KEY" -out client.csr -nodes -subj "$CLIENT_SUBJ"
    openssl x509 -req -in client.csr -CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial -out "$TARGET_CERT" -days "$DAYS_VALID"
    rm -f client.csr
    echo "✅ Emergency certificate generated successfully."
    exit 0
fi

# 2. Extract remaining time using OpenSSL checkend (measures in seconds)
# THRESHOLD_DAYS * 24 hours * 60 minutes * 60 seconds
THRESHOLD_SECONDS=$(( THRESHOLD_DAYS * 24 * 60 * 60 ))

if openssl x509 -checkend "$THRESHOLD_SECONDS" -noout -in "$TARGET_CERT"; then
    # Certificate is still valid beyond the threshold duration
    EXP_DATE=$(openssl x509 -enddate -noout -in "$TARGET_CERT" | cut -d= -f2)
    echo "✅ Certificate is healthy. It expires on: ${EXP_DATE}. No action needed."
    exit 0
else
    echo "⚠️  Certificate is expiring in less than ${THRESHOLD_DAYS} days! Starting rotation..."

    # Define temporary files so we don't overwrite live assets mid-flight
    NEXT_CERT="client.crt.next"
    NEXT_KEY="client.key.next"
    TEMP_CSR="client.csr.tmp"

    # 3. Generate a brand new keypair and CSR
    openssl req -newkey rsa:2048 -keyout "$NEXT_KEY" -out "$TEMP_CSR" -nodes -subj "$CLIENT_SUBJ"

    # 4. Sign the new certificate using our existing local private CA
    openssl x509 -req -in "$TEMP_CSR" -CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial -out "$NEXT_CERT" -days "$DAYS_VALID"

    # 5. Atomic swap to replace old production files instantly
    mv "$NEXT_KEY" "$TARGET_KEY"
    mv "$NEXT_CERT" "$TARGET_CERT"

    # Clean up compilation leftovers
    rm -f "$TEMP_CSR"

    echo "🎉 Success! New keys deployed. Valid for the next ${DAYS_VALID} days."

    # 6. SIGNAL APP SIGNALING (Optional but highly recommended)
    # If using Docker or systemd, you can tell the service to reload files here.

    # TODO
    # docker kill -s HUP slurm-security-gateway || true
fi
