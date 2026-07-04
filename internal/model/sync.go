package model

// SyncRequest is the optional JSON body for POST /api/sync.
type SyncRequest struct {
	Assets  []string `json:"assets,omitempty"`
	Days    int      `json:"days,omitempty"`
	Workers int      `json:"workers,omitempty"`
	Delay   int      `json:"delay,omitempty"`
}

// SyncResponse is returned on 202 Accepted after starting a sync.
type SyncResponse struct {
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	Assets    []string `json:"assets,omitempty"`
	Days      int      `json:"days,omitempty"`
	Workers   int      `json:"workers,omitempty"`
	Timestamp string   `json:"timestamp"`
}

// ErrorResponse is returned on 4xx and 5xx errors.
type ErrorResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}
