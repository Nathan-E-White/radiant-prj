
package main

// JobSubmitRequest defines the incoming payload from the React frontend
type JobSubmitRequest struct {
	// NodeCount must be an integer between 1 and 32
	NodeCount int `json:"node_count" validate:"required,gte=1,lte=32"`

	// Partition must strictly match one of your allowed cluster queues
	Partition string `json:"partition" validate:"required,oneof=cpu-short cpu-long gpu"`

	// ScriptName must contain only alphanumeric characters and dashes (no paths, slashes, or dots)
	ScriptName string `json:"script_name" validate:"required,alphanum,max=30"`
}

// JobResponse is sent back to the React client
type JobResponse struct {
	Message string `json:"message"`
	JobID   string `json:"job_id,omitempty"`
	Error   string `json:"error,omitempty"`
}