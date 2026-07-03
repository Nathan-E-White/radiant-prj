package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)


var validate = validator.New()


import (
	"encoding/json"
	"log"
	"net/http"
)

package main

import (
	"crypto/x509"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"://github.com"
	"://github.com/promauto"
)

// 1. Declare Custom Prometheus Performance Metrics
var (
	jobsSubmittedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "slurm_gateway_jobs_submitted_total",
			Help: "The total number of submitted batch jobs to the Slurm compute cluster",
		},
		[]string{"client_identity", "status"}, // Multi-dimensional labeling
	)

	certExpirationGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "slurm_gateway_cert_expiry_days_remaining",
			Help: "The number of days remaining before the active mTLS certificate expires",
		},
	)
)

// TrackCertExpiry calculates time remaining and updates our Prometheus monitoring loop
func TrackCertExpiry(cert *x509.Certificate) {
	go func() {
		for {
			remaining := time.Until(cert.NotAfter)
			days := remaining.Hours() / 24
			certExpirationGauge.Set(days)

			// Re-evaluate every 1 hour
			time.Sleep(1 * time.Hour)
		}
	}()
}

func HandleSubmitJobWithMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Verify mTLS identity state
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		jobsSubmittedCounter.WithLabelValues("anonymous", "unauthorized").Inc()
		json.NewEncoder(w).Encode(JobResponse{Error: "mTLS client certificate missing"})
		return
	}

	clientIdentity := r.TLS.PeerCertificates[0].Subject.CommonName

	// Pass data into our background monitoring metrics
	// If the job executes successfully later in your existing routine:
	jobsSubmittedCounter.WithLabelValues(clientIdentity, "success").Inc()

	// (Rest of your existing sbatch verification and exec pipeline follows here...)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JobResponse{Message: "Job queued successfully", JobID: "12345"})
}


func HandleSubmitJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Extract the client certificate from the TLS connection state
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(JobResponse{Error: "mTLS client certificate missing"})
		return
	}

	// The first certificate in the chain is always the client's leaf certificate
	clientCert := r.TLS.PeerCertificates[0]

	// 2. Extract identity fields (e.g., Common Name)
	clientIdentity := clientCert.Subject.CommonName
	if clientIdentity == "" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(JobResponse{Error: "Certificate Common Name (CN) is empty"})
		return
	}

	// 3. Use this identity for Auditing and Access Control
	log.Printf("[AUDIT] Slurm job request received from verified client identity: %s", clientIdentity)

	// Optional: Enforce authorization limits based on identity
	if clientIdentity != "react-backend-client" && clientIdentity != "cluster-admin" {
		w.WriteHeader(http.StatusForbidden)
		log.Printf("[SECURITY ALERT] Unauthorized identity '%s' attempted to submit a job", clientIdentity)
		json.NewEncoder(w).Encode(JobResponse{Error: "Your certificate identity is not authorized to submit Slurm jobs"})
		return
	}

	// ----------------------------------------------------
	// ... Continue with your existing input decoding,
	// validation, and exec.Command("sbatch", ...) logic ...
	// ----------------------------------------------------
}

/*
func HandleSubmitJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(JobResponse{Error: "Only POST requests allowed"})
		return
	}

	// 1. Decode incoming JSON
	var req JobSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JobResponse{Error: "Invalid JSON payload"})
		return
	}

	// 2. Run Validation Rules
	if err := validate.Struct(req); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(JobResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// 3. SECURE EXECUTION
	// Constructing the full path locally ensures the user can't traverse directories (e.g., ../../etc/passwd)
	safeScriptPath := "/opt/slurm/scripts/" + req.ScriptName + ".sh"

	// Transform integer node count securely to a string flag
	nodesFlag := "--nodes=" + strconv.Itoa(req.NodeCount)
	partitionFlag := "--partition=" + req.Partition

	// exec.Command avoids invoking /bin/sh. Arguments are isolated OS vectors.
	cmd := exec.Command("sbatch", nodesFlag, partitionFlag, safeScriptPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute binary
	if err := cmd.Run(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(JobResponse{
			Error: "Slurm cluster execution failed: " + stderr.String(),
		})
		return
	}

	// Parse Slurm's standard output (e.g., "Submitted batch job 12345")
	outputStr := strings.TrimSpace(stdout.String())

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JobResponse{
		Message: "Job successfully queued on cluster",
		JobID:   outputStr,
	})
}
*/