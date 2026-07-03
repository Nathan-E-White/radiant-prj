package main

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	vault "://github.com"
)

// FetchCertificateFromVault authenticates with Vault and requests a fresh 24h mTLS certificate pair
func FetchCertificateFromVault() (tls.Certificate, error) {
	// Initialize default configuration (reads VAULT_ADDR and VAULT_TOKEN environment variables)
	config := vault.DefaultConfig()
	client, err := vault.NewClient(config)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Prepare request payload for our defined Vault role
	data := map[string]interface{}{
		"common_name": "localhost",
		"ttl":         "24h",
	}

	// Issue the issue request to Vault's PKI engine path
	secret, err := client.KVv2("pki").Get(context.Background(), "issue/slurm-cluster-nodes") // Adjusted for modern KV v2 syntax matching role paths
	// Note: For raw path writes in standard Vault PKI paths, use client.Logical().Write():
	secret, err = client.Logical().Write("pki/issue/slurm-cluster-nodes", data)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Extract the text-encoded strings from the Vault response map
	certPEM := secret.Data["certificate"].(string)
	keyPEM := secret.Data["private_key"].(string)

	// Compile the raw text directly into an operational Go memory structure
	return tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
}

// BackgroundRotationLoop fetches a fresh certificate every 12 hours before expiration hits
func BackgroundRotationLoop(currentCert *tls.Certificate) {
	ticker := time.NewTicker(12 * time.Hour)
	for range ticker.C {
		log.Println("[VAULT] Initiating background automated certificate renewal...")
		newCert, err := FetchCertificateFromVault()
		if err != nil {
			log.Printf("[ERROR] Background Vault renewal failed: %v", err)
			continue
		}
		*currentCert = newCert
		log.Println("[VAULT] Successfully renewed short-lived mTLS certificate.")
	}
}
