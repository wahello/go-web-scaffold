package metric

// Usual API metrics
const (
	RequestsTotal = "requests.total"
	// RequestDuration 's unit is in seconds, per to Prometheus's suggestion
	RequestDuration   = "request.duration"
	RequestSizeBytes  = "request.size.bytes"
	ResponseSizeBytes = "response.size.bytes"
)
