package metrics

import (
	"fmt"

	"github.com/moonfrog/badger/logs"

	"time"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

func LogPeriodic(interval time.Duration) {
	LogPeriodicRegistry(DefaultRegistry, interval)
}

func LogPeriodicRegistry(r Registry, interval time.Duration) {
	for _ = range time.Tick(interval) {
		r.Each(func(name string, m interface{}) {
			val := ""
			switch metric := m.(type) {
			case Counter:
				val = fmt.Sprintf("%d", metric.Count())
			case Gauge:
				val = fmt.Sprintf("%d", metric.Value())
			case GaugeFloat64:
				val = fmt.Sprintf("%f", metric.Value())
			case Healthcheck:
				metric.Check()
				val = fmt.Sprintf("%v", metric.Error())
			case Histogram:
				h := metric.Snapshot()
				ps := h.Percentiles([]float64{0.5, 0.80, 0.95, 0.99, 0.999})
				val = fmt.Sprintf("count: %d, min: %d, max: %d, mean: %f, stddev: %f, median: %f, 80%%: %f, 95%%: %f, 99%%: %f, 99.9%%: %f",
					h.Count(), h.Min(), h.Max(), h.Mean(), h.StdDev(), ps[0], ps[1], ps[2], ps[3], ps[4])
			case Meter:
				m := metric.Snapshot()
				val = fmt.Sprintf("count: %d, 1MR: %f, 5MR: %f, 15MR: %f, mean: %f", m.Count(), m.Rate1(), m.Rate5(), m.Rate15(), m.RateMean())
			case Timer:
				scale := float64(time.Second)
				t := metric.Snapshot()
				ps := t.Percentiles([]float64{0.5, 0.80, 0.95, 0.99, 0.999})
				val = fmt.Sprintf("count: %d, min: %f, max: %f, mean: %f, stddev: %f, median: %f, 80%%: %f, 95%%: %f, 99%%: %f, 99.9%%: %f 1MR: %f, 5MR: %f, 15MR: %f, meanRate: %f", t.Count(), float64(t.Min())/scale, float64(t.Max())/scale, t.Mean()/scale, t.StdDev()/scale, ps[0]/scale, ps[1]/scale, ps[2]/scale, ps[3]/scale, ps[4]/scale, t.Rate1(), t.Rate5(), t.Rate15(), t.RateMean())
			}

			logs.Info("Metrics: %s: %v", name, val)
		})
	}
}

func Log(r Registry, freq time.Duration, l Logger) {
	LogScaled(r, freq, time.Nanosecond, l)
}

// Output each metric in the given registry periodically using the given
// logger. Print timings in `scale` units (eg time.Millisecond) rather than nanos.
func LogScaled(r Registry, freq time.Duration, scale time.Duration, l Logger) {
	du := float64(scale)
	duSuffix := scale.String()[1:]

	for _ = range time.Tick(freq) {
		r.Each(func(name string, i interface{}) {
			switch metric := i.(type) {
			case Counter:
				l.Printf("counter %s\n", name)
				l.Printf("  count:       %9d\n", metric.Count())
			case Gauge:
				l.Printf("gauge %s\n", name)
				l.Printf("  value:       %9d\n", metric.Value())
			case GaugeFloat64:
				l.Printf("gauge %s\n", name)
				l.Printf("  value:       %f\n", metric.Value())
			case Healthcheck:
				metric.Check()
				l.Printf("healthcheck %s\n", name)
				l.Printf("  error:       %v\n", metric.Error())
			case Histogram:
				h := metric.Snapshot()
				ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
				l.Printf("histogram %s\n", name)
				l.Printf("  count:       %9d\n", h.Count())
				l.Printf("  min:         %9d\n", h.Min())
				l.Printf("  max:         %9d\n", h.Max())
				l.Printf("  mean:        %12.2f\n", h.Mean())
				l.Printf("  stddev:      %12.2f\n", h.StdDev())
				l.Printf("  median:      %12.2f\n", ps[0])
				l.Printf("  75%%:         %12.2f\n", ps[1])
				l.Printf("  95%%:         %12.2f\n", ps[2])
				l.Printf("  99%%:         %12.2f\n", ps[3])
				l.Printf("  99.9%%:       %12.2f\n", ps[4])
			case Meter:
				m := metric.Snapshot()
				l.Printf("meter %s\n", name)
				l.Printf("  count:       %9d\n", m.Count())
				l.Printf("  1-min rate:  %12.2f\n", m.Rate1())
				l.Printf("  5-min rate:  %12.2f\n", m.Rate5())
				l.Printf("  15-min rate: %12.2f\n", m.Rate15())
				l.Printf("  mean rate:   %12.2f\n", m.RateMean())
			case Timer:
				t := metric.Snapshot()
				ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
				l.Printf("timer %s\n", name)
				l.Printf("  count:       %9d\n", t.Count())
				l.Printf("  min:         %12.2f%s\n", float64(t.Min())/du, duSuffix)
				l.Printf("  max:         %12.2f%s\n", float64(t.Max())/du, duSuffix)
				l.Printf("  mean:        %12.2f%s\n", t.Mean()/du, duSuffix)
				l.Printf("  stddev:      %12.2f%s\n", t.StdDev()/du, duSuffix)
				l.Printf("  median:      %12.2f%s\n", ps[0]/du, duSuffix)
				l.Printf("  75%%:         %12.2f%s\n", ps[1]/du, duSuffix)
				l.Printf("  95%%:         %12.2f%s\n", ps[2]/du, duSuffix)
				l.Printf("  99%%:         %12.2f%s\n", ps[3]/du, duSuffix)
				l.Printf("  99.9%%:       %12.2f%s\n", ps[4]/du, duSuffix)
				l.Printf("  1-min rate:  %12.2f\n", t.Rate1())
				l.Printf("  5-min rate:  %12.2f\n", t.Rate5())
				l.Printf("  15-min rate: %12.2f\n", t.Rate15())
				l.Printf("  mean rate:   %12.2f\n", t.RateMean())
			}
		})
	}
}
