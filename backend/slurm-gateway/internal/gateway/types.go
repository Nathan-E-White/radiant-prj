package gateway

import "time"

type JobState string

const (
	StateQueued JobState = "queued"
	StateFailed JobState = "failed"
)

type SubmitRequest struct {
	ScriptName string `json:"script_name"`
	Partition  string `json:"partition"`
	NodeCount  int    `json:"node_count"`
	RankCount  int    `json:"rank_count,omitempty"`
}

type SubmitResponse struct {
	Message string   `json:"message"`
	JobID   string   `json:"job_id"`
	State   JobState `json:"state"`
	Mode    string   `json:"mode"`
}

type StatusResponse struct {
	JobID       string    `json:"job_id"`
	State       JobState  `json:"state"`
	Mode        string    `json:"mode"`
	ScriptName  string    `json:"script_name"`
	Partition   string    `json:"partition"`
	NodeCount   int       `json:"node_count"`
	RankCount   int       `json:"rank_count"`
	SubmittedBy string    `json:"submitted_by"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type JobRecord struct {
	JobID       string
	State       JobState
	Mode        string
	ScriptName  string
	Partition   string
	NodeCount   int
	RankCount   int
	SubmittedBy string
	SubmittedAt time.Time
}

func (r JobRecord) StatusResponse() StatusResponse {
	return StatusResponse{
		JobID:       r.JobID,
		State:       r.State,
		Mode:        r.Mode,
		ScriptName:  r.ScriptName,
		Partition:   r.Partition,
		NodeCount:   r.NodeCount,
		RankCount:   r.RankCount,
		SubmittedBy: r.SubmittedBy,
		SubmittedAt: r.SubmittedAt,
	}
}
