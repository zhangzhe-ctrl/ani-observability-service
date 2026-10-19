package monitoring

import "fmt"

const (
	InvalidQuery          = "MONITORING_INVALID_QUERY"
	ResultTooLarge        = "MONITORING_RESULT_TOO_LARGE"
	ConcurrencyLimit      = "MONITORING_CONCURRENCY_LIMIT_EXCEEDED"
	SourceNotConfigured   = "MONITORING_SOURCE_NOT_CONFIGURED"
	SourceUnavailable     = "MONITORING_SOURCE_UNAVAILABLE"
	SourceResponseInvalid = "MONITORING_SOURCE_RESPONSE_INVALID"
	QueryFailed           = "MONITORING_QUERY_FAILED"
	QueryTimeout          = "MONITORING_QUERY_TIMEOUT"
)

type Failure struct {
	Reason string
	Cause  error
}

func (f *Failure) Error() string            { return fmt.Sprintf("%s: %v", f.Reason, f.Cause) }
func (f *Failure) Unwrap() error            { return f.Cause }
func Fail(reason string, cause error) error { return &Failure{reason, cause} }
