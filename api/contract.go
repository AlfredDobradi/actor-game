package api

type BuildRequest struct {
	Blueprint string `json:"blueprint"`
}

type BuildResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
