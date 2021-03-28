package metrics

import (
	"go.uber.org/atomic"
)

// RestAPIMetrics defines REST API metrics over the entire runtime of the node.
type RestAPIMetrics struct {
	// The total number HTTP request errors.
	HTTPRequestErrorCounter atomic.Uint32
}
