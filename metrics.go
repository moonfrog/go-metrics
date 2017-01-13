// Go port of Coda Hale's Metrics library
//
// <https://github.com/rcrowley/go-metrics>
//
// Coda Hale's original work: <https://github.com/codahale/metrics>
package metrics

import "time"

// UseNilMetrics is checked by the constructor functions for all of the
// standard metrics.  If it is true, the metric returned is a stub.
//
// This global kill-switch helps quantify the observer effect and makes
// for less cluttered pprof profiles.
var UseNilMetrics bool = false

var TimerWindow int = 100000

var MeterRescaleThreshold time.Duration = 5 * time.Minute

// names for general metrics
const (
	RSTAT_ERROR = "error"
	RSTAT_WARN  = "warn"
	RSTAT_PANIC = "panic"
)
