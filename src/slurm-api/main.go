package main

import (
	"log"
	"net/http"
)

// enableCORS acts as a security middleware to safely allow cross-origin React requests
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set allowed origins. In production, change "*" to your exact React domain
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight OPTIONS request from browsers instantly
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Pass the request to our actual handler logic
		next(w, r)
	}
}

func main() {
	// Wrap our endpoint with the CORS middleware
	http.HandleFunc("/api/jobs/submit", enableCORS(HandleSubmitJob))

	log.Println("Server securely starting on :8080 with CORS enabled...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

		// Business route for job submission
		// TODO This is an example
    	http.HandleFunc("/api/jobs/submit", HandleSubmitJobWithMetrics)

    	// Expose standard Prometheus metric scraping interface
    	http.Handle("/metrics", promhttp.Handler())

    	log.Println("Server executing with Prometheus metric endpoints exposed at :8080/metrics")
    	log.Fatal(http.ListenAndServe(":8080", nil))
}
