package metrics

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// DuplicateMetric is the error returned by Registry.Register when a metric
// already exists.  If you mean to Register that metric you must first
// Unregister the existing metric.
type DuplicateMetric string

func (err DuplicateMetric) Error() string {
	return fmt.Sprintf("duplicate metric: %s", string(err))
}

// A Registry holds references to a set of metrics by name and can iterate
// over them, calling callback functions provided by the user.
//
// This is an interface so as to encourage other structs to implement
// the Registry API as appropriate.
type Registry interface {

	// Call the given function for each registered metric.
	Each(func(string, interface{}))

	// Get the metric by the given name or nil if none is registered.
	Get(string) interface{}

	// Gets an existing metric or registers the given one.
	// The interface can be the metric to register if not found in registry,
	// or a function returning the metric for lazy instantiation.
	GetOrRegister(string, interface{}) interface{}

	// Register the given metric under the given name.
	Register(string, interface{}) error

	// Run all registered healthchecks.
	RunHealthchecks()

	// Unregister the metric with the given name.
	Unregister(string)

	// Unregister all metrics.  (Mostly for testing.)
	UnregisterAll()

	// updates the metric name with val, doesn't work for gaugeFloat64
	Update(name string, val int64)

	// current stats string
	GetCurrent() string
}

// The standard implementation of a Registry is a mutex-protected map
// of names to metrics.
type StandardRegistry struct {
	metrics map[string]Metric
	mutex   sync.RWMutex
}

// Create a new registry.
func NewRegistry() Registry {
	return &StandardRegistry{metrics: make(map[string]Metric)}
}

// Call the given function for each registered metric.
func (r *StandardRegistry) Each(f func(string, interface{})) {
	registeredMetrics := r.registered()
	keys := make([]string, 0, len(registeredMetrics))
	for name, _ := range registeredMetrics {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	for _, name := range keys {
		f(name, r.registered()[name])
	}
}

// Get the metric by the given name or nil if none is registered.
func (r *StandardRegistry) Get(name string) interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.metrics[name]
}

// Gets an existing metric or creates and registers a new one. Threadsafe
// alternative to calling Get and Register on failure.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *StandardRegistry) GetOrRegister(name string, i interface{}) interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if metric, ok := r.metrics[name]; ok {
		return metric
	}
	if v := reflect.ValueOf(i); v.Kind() == reflect.Func {
		i = v.Call(nil)[0].Interface()
	}
	r.register(name, i)
	return i
}

// creates a counter if metric doesn't exist
func (r *StandardRegistry) Update(name string, val int64) {
	r.mutex.RLock()
	m := r.metrics[name]
	r.mutex.RUnlock()
	if m == nil {
		m = NewRegisteredCounter(name, r)
	}

	m.Update(val)
}

// Register the given metric under the given name.  Returns a DuplicateMetric
// if a metric by the given name is already registered.
func (r *StandardRegistry) Register(name string, i interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.register(name, i)
}

// Run all registered healthchecks.
func (r *StandardRegistry) RunHealthchecks() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, i := range r.metrics {
		if h, ok := i.(Healthcheck); ok {
			h.Check()
		}
	}
}

// Unregister the metric with the given name.
func (r *StandardRegistry) Unregister(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.metrics, name)
}

// Unregister all metrics.  (Mostly for testing.)
func (r *StandardRegistry) UnregisterAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for name, _ := range r.metrics {
		delete(r.metrics, name)
	}
}

// assumes lock is taken
func (r *StandardRegistry) register(name string, i interface{}) error {
	if _, ok := r.metrics[name]; ok {
		return DuplicateMetric(name)
	}
	switch i.(type) {
	// TODO: add gaugefloat
	case Counter, Gauge, Healthcheck, Histogram, Meter, Timer, Instant:
		r.metrics[name] = i.(Metric)
	case GaugeFloat64:
		// TODO: fix
		r.metrics[name] = NewGauge()
	}
	return nil
}

func (r *StandardRegistry) registered() map[string]Metric {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	metrics := make(map[string]Metric, len(r.metrics))
	for name, i := range r.metrics {
		metrics[name] = i
	}
	return metrics
}

func (r *StandardRegistry) GetCurrent() string {
	result := "<--------Metrics--------->\n"
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
			ps := h.Percentiles([]float64{0.5, 0.80, 0.90, 0.99, 0.999})
			val = fmt.Sprintf("count: %d, min: %d, max: %d, mean: %f, stddev: %f, median: %f, 80%%: %f, 90%%: %f, 99%%: %f, 99.9%%: %f",
				h.Count(), h.Min(), h.Max(), h.Mean(), h.StdDev(), ps[0], ps[1], ps[2], ps[3], ps[4])
		case Meter:
			m := metric.Snapshot()
			val = fmt.Sprintf("count: %d, 1MR: %f, 5MR: %f, 15MR: %f, mean: %f", m.Count(), m.Rate1(), m.Rate5(), m.Rate15(), m.RateMean())
		case Timer:
			scale := float64(time.Second)
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.80, 0.90, 0.99, 0.999})
			val = fmt.Sprintf("count: %d, min: %f, max: %f, mean: %f, stddev: %f, median: %f, 80%%: %f, 90%%: %f, 99%%: %f, 99.9%%: %f 1MR: %f, 5MR: %f, 15MR: %f, meanRate: %f", t.Count(), float64(t.Min())/scale, float64(t.Max())/scale, t.Mean()/scale, t.StdDev()/scale, ps[0]/scale, ps[1]/scale, ps[2]/scale, ps[3]/scale, ps[4]/scale, t.Rate1(), t.Rate5(), t.Rate15(), t.RateMean())
		}

		result += fmt.Sprintf("Metrics: %s: %v\n", name, val)
	})

	return result
}

type PrefixedRegistry struct {
	underlying Registry
	prefix     string
}

func NewPrefixedRegistry(prefix string) Registry {
	return &PrefixedRegistry{
		underlying: NewRegistry(),
		prefix:     prefix,
	}
}

func NewPrefixedChildRegistry(parent Registry, prefix string) Registry {
	return &PrefixedRegistry{
		underlying: parent,
		prefix:     prefix,
	}
}

// Call the given function for each registered metric.
func (r *PrefixedRegistry) Each(fn func(string, interface{})) {
	wrappedFn := func(prefix string) func(string, interface{}) {
		return func(name string, iface interface{}) {
			if strings.HasPrefix(name, prefix) {
				fn(name, iface)
			} else {
				return
			}
		}
	}

	baseRegistry, prefix := findPrefix(r, "")
	baseRegistry.Each(wrappedFn(prefix))
}

func (r *PrefixedRegistry) Update(name string, val int64) {
	r.underlying.Update(name, val)
}

func findPrefix(registry Registry, prefix string) (Registry, string) {
	switch r := registry.(type) {
	case *PrefixedRegistry:
		return findPrefix(r.underlying, r.prefix+prefix)
	case *StandardRegistry:
		return r, prefix
	}
	return nil, ""
}

// Get the metric by the given name or nil if none is registered.
func (r *PrefixedRegistry) Get(name string) interface{} {
	realName := r.prefix + name
	return r.underlying.Get(realName)
}

// Gets an existing metric or registers the given one.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *PrefixedRegistry) GetOrRegister(name string, metric interface{}) interface{} {
	realName := r.prefix + name
	return r.underlying.GetOrRegister(realName, metric)
}

// Register the given metric under the given name. The name will be prefixed.
func (r *PrefixedRegistry) Register(name string, metric interface{}) error {
	realName := r.prefix + name
	return r.underlying.Register(realName, metric)
}

// Run all registered healthchecks.
func (r *PrefixedRegistry) RunHealthchecks() {
	r.underlying.RunHealthchecks()
}

// Unregister the metric with the given name. The name will be prefixed.
func (r *PrefixedRegistry) Unregister(name string) {
	realName := r.prefix + name
	r.underlying.Unregister(realName)
}

// Unregister all metrics.  (Mostly for testing.)
func (r *PrefixedRegistry) UnregisterAll() {
	r.underlying.UnregisterAll()
}

func (r *PrefixedRegistry) GetCurrent() string {
	return r.underlying.GetCurrent()
}

var DefaultRegistry Registry = NewRegistry()

// Call the given function for each registered metric.
func Each(f func(string, interface{})) {
	DefaultRegistry.Each(f)
}

// Get the metric by the given name or nil if none is registered.
func Get(name string) interface{} {
	return DefaultRegistry.Get(name)
}

// Gets an existing metric or creates and registers a new one. Threadsafe
// alternative to calling Get and Register on failure.
func GetOrRegister(name string, i interface{}) interface{} {
	return DefaultRegistry.GetOrRegister(name, i)
}

// Register the given metric under the given name.  Returns a DuplicateMetric
// if a metric by the given name is already registered.
func Register(name string, i interface{}) error {
	return DefaultRegistry.Register(name, i)
}

// Register the given metric under the given name.  Panics if a metric by the
// given name is already registered.
func MustRegister(name string, i interface{}) {
	if err := Register(name, i); err != nil {
		panic(err)
	}
}

// Run all registered healthchecks.
func RunHealthchecks() {
	DefaultRegistry.RunHealthchecks()
}

// Unregister the metric with the given name.
func Unregister(name string) {
	DefaultRegistry.Unregister(name)
}

func Update(name string, val int64) {
	DefaultRegistry.Update(name, val)
}

func GetCurrent() string {
	return DefaultRegistry.GetCurrent()
}
