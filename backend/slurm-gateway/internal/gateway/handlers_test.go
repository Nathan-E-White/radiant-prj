package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubmitRequiresClientCertificate(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/submit", strings.NewReader(validSubmitBody()))
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHealthAndReadyHandlers(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		app.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("%s expected ok, got %d: %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestReadyFailsInSbatchModeWithoutScriptRoot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeSbatch
	cfg.ScriptRoot = t.TempDir() + "/missing"
	app := NewGateway(cfg, MockSpooler{}, NewInMemoryJobStore(), NewMetrics())

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLifecycleReadinessStartsBeforeTheInitialCycleAndKeepsLivenessAvailable(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})
	app.EnableLifecycleHealth()

	for path, want := range map[string]int{"/healthz": http.StatusOK, "/readyz": http.StatusServiceUnavailable} {
		rr := httptest.NewRecorder()
		app.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != want {
			t.Fatalf("%s status=%d want=%d", path, rr.Code, want)
		}
	}
	app.lifecycle.RecordOutcomes(time.Now(), nil)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("ready after successful initial cycle: %d", rr.Code)
	}
}

func TestLifecycleShutdownCancellationDoesNotOverwriteLastHealthyCycle(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})
	app.EnableLifecycleHealth()
	app.lifecycle.RecordOutcomes(time.Now(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.ReconcileFleetBoardSessions(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown cancellation: %v", err)
	}
	if got := app.lifecycle.State(); got != LifecycleReady {
		t.Fatalf("shutdown cancellation changed lifecycle state: %s", got)
	}
}

func TestLifecycleDeadlineMarksTheInitialCycleNotReady(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})
	app.EnableLifecycleHealth()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := app.ReconcileFleetBoardSessions(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline expiry: %v", err)
	}
	if got := app.lifecycle.State(); got != LifecycleNotReady {
		t.Fatalf("deadline did not mark initial lifecycle state not ready: %s", got)
	}
}

func TestLifecycleRequiredTaskBecomesNotReadyAfterItsSuccessWindow(t *testing.T) {
	health := NewLifecycleHealth(true)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	health.RecordOutcomes(now, []LifecycleTaskOutcome{{Name: "measured_retention", Err: nil}})
	health.RecordOutcomes(now.Add(time.Second), []LifecycleTaskOutcome{{Name: "measured_retention", Err: errors.New("storage unavailable")}})
	if got := health.StateAt(now.Add(time.Minute)); got != LifecycleDegraded {
		t.Fatalf("recent failed task should degrade: %s", got)
	}
	if got := health.StateAt(now.Add(3 * time.Minute)); got != LifecycleNotReady {
		t.Fatalf("overdue failed task should be not ready: %s", got)
	}
}

func TestSubmitRejectsUnauthorizedIdentity(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := signedRequest(http.MethodPost, "/api/jobs/submit", validSubmitBody(), "some-random-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitRejectsBadMethod(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := signedRequest(http.MethodGet, "/api/jobs/submit", "", "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitRejectsBadJSON(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := signedRequest(http.MethodPost, "/api/jobs/submit", `{"script_name":`, "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitRejectsValidationFailure(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	body := `{"script_name":"../../etc/passwd","partition":"transport","node_count":1,"rank_count":1}`
	req := signedRequest(http.MethodPost, "/api/jobs/submit", body, "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMockSubmitAndStatusLookup(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})
	app.now = func() time.Time {
		return time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	}

	submit := signedRequest(http.MethodPost, "/api/jobs/submit", validSubmitBody(), "react-backend-client")
	submitRR := httptest.NewRecorder()

	app.Handler().ServeHTTP(submitRR, submit)

	if submitRR.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d: %s", submitRR.Code, submitRR.Body.String())
	}

	var submitResponse SubmitResponse
	if err := json.Unmarshal(submitRR.Body.Bytes(), &submitResponse); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if !strings.HasPrefix(submitResponse.JobID, "MOCK-") {
		t.Fatalf("expected mock job id, got %q", submitResponse.JobID)
	}
	if submitResponse.State != StateQueued {
		t.Fatalf("expected queued state, got %q", submitResponse.State)
	}

	status := signedRequest(http.MethodGet, "/api/jobs/"+submitResponse.JobID, "", "react-backend-client")
	statusRR := httptest.NewRecorder()

	app.Handler().ServeHTTP(statusRR, status)

	if statusRR.Code != http.StatusOK {
		t.Fatalf("expected status ok, got %d: %s", statusRR.Code, statusRR.Body.String())
	}

	var statusResponse StatusResponse
	if err := json.Unmarshal(statusRR.Body.Bytes(), &statusResponse); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if statusResponse.JobID != submitResponse.JobID {
		t.Fatalf("expected status for %q, got %q", submitResponse.JobID, statusResponse.JobID)
	}
	if statusResponse.SubmittedBy != "react-backend-client" {
		t.Fatalf("expected submitting identity to be recorded")
	}
}

func TestStatusMissingJobReturnsNotFound(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := signedRequest(http.MethodGet, "/api/jobs/MOCK-missing", "", "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitSpoolerFailure(t *testing.T) {
	app := newTestGateway(t, failingSpooler{})

	req := signedRequest(http.MethodPost, "/api/jobs/submit", validSubmitBody(), "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected bad gateway, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMetricsIncludeSubmitCountsWithoutPayloadLabels(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})

	req := signedRequest(http.MethodPost, "/api/jobs/submit", validSubmitBody(), "react-backend-client")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	metrics := httptest.NewRecorder()
	app.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := metrics.Body.String()
	if !strings.Contains(body, `slurm_gateway_job_submit_total{status="success",mode="mock"} 1`) {
		t.Fatalf("expected success counter, got:\n%s", body)
	}
	if strings.Contains(body, "script_name") || strings.Contains(body, "transport-toy") {
		t.Fatalf("metrics leaked request payload:\n%s", body)
	}
}

func TestLifecycleMetricsHaveFixedTaskCardinality(t *testing.T) {
	app := newTestGateway(t, MockSpooler{})
	app.EnableLifecycleHealth()
	metrics := httptest.NewRecorder()
	app.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metrics.Body.String()
	for _, task := range lifecycleTaskNames {
		if !strings.Contains(body, `slurm_gateway_lifecycle_task_success{task="`+task+`"}`) ||
			!strings.Contains(body, `slurm_gateway_lifecycle_task_success_age_seconds{task="`+task+`"}`) ||
			!strings.Contains(body, `slurm_gateway_lifecycle_task_affected_count{task="`+task+`"}`) {
			t.Fatalf("missing fixed lifecycle metrics for %s:\n%s", task, body)
		}
	}
}

type failingSpooler struct{}

func (failingSpooler) Submit(ctx context.Context, req SubmitRequest, identity string) (SubmitResult, error) {
	return SubmitResult{}, errors.New("boom")
}

func newTestGateway(t *testing.T, spooler SlurmSpooler) *Gateway {
	t.Helper()
	cfg := DefaultConfig()
	cfg.RequestTimeout = time.Second
	app := NewGateway(cfg, spooler, NewInMemoryJobStore(), NewMetrics())
	return app
}

func signedRequest(method string, path string, body string, commonName string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{
			Subject: pkix.Name{CommonName: commonName},
		}},
	}
	return req
}

func validSubmitBody() string {
	return `{"script_name":"transport-toy","partition":"transport","node_count":2,"rank_count":8}`
}
