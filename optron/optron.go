package optron

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/moonfrog/badger/utils"
	"github.com/moonfrog/go-metrics"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

type Optron struct {
	name     string
	game     string
	config   *ConfigOptronDef
	conn     *net.TCPConn
	interval time.Duration
	working  bool
	l        Logger
	builder  *OptronObjBuilder
}

type OptronObjBuilder struct {
	hasBulkSupport bool
	batchSize      int
	standaloneObj  map[string]interface{}
	objList        []map[string]interface{}
}

func (ob *OptronObjBuilder) append(data map[string]interface{}) {
	if ob.hasBulkSupport {
		ob.objList = append(ob.objList, data)
	} else {
		for k, v := range data {
			ob.standaloneObj[k] = v
		}
	}
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func (ob *OptronObjBuilder) flush() []interface{} {
	defer func() {
		ob.standaloneObj = make(map[string]interface{})
		ob.objList = []map[string]interface{}{}
	}()

	if ob.hasBulkSupport {
		var data []interface{}
		for i := 0; i < len(ob.objList); i += ob.batchSize {
			data = append(data, ob.objList[i:min(i+ob.batchSize, len(ob.objList))])
		}
		return data
	} else {
		return []interface{}{ob.standaloneObj}
	}
}

func (this *Optron) init(configUri string) error {
	var err error
	this.config, err = getOptronConfig(configUri)
	if err != nil {
		return fmt.Errorf("optron config: get: %v", err)
	}

	if this.config.HasBulkSupport && this.config.BatchSize < 1 {
		return fmt.Errorf("optron config: Invalid batch size: %v", this.config.BatchSize)
	}

	this.builder = &OptronObjBuilder{
		hasBulkSupport: this.config.HasBulkSupport,
		standaloneObj:  make(map[string]interface{}),
	}
	return nil
}

func (this *Optron) Start() {
	for range time.Tick(this.interval) {
		this.send()
	}
}

func (this *Optron) connect() {
	this.working = false
	this.l.Printf("Connecting to : %v\n", this.config.Address)
	conn, err := net.Dial("tcp", this.config.Address)
	if err != nil {
		this.l.Printf("Warn: optron: connect: %v", err)
	} else {
		this.conn = conn.(*net.TCPConn)
		this.working = true
	}
}

func (this *Optron) send() {
	if !this.working {
		this.connect()
		if !this.working {
			return
		}
	}

	metrics.DefaultRegistry.Each(func(name string, m interface{}) {

		optronObj := map[string]interface{}{
			"hostName": utils.GetIpAddress(),
			"id":       this.name,
			"game":     this.game}

		if metrics.IsTagged(name) {
			var tagMap map[string]string
			name, tagMap = metrics.ParseTaggedMetric(name)
			for k, v := range tagMap {
				optronObj[k] = v
			}
		}

		switch metric := m.(type) {
		case metrics.Instant:
			optronObj[name] = metric.Count()
			metric.Clear()
		case metrics.Counter:
			optronObj[name] = metric.Count()
		case metrics.Gauge:
			optronObj[name] = metric.Value()
		case metrics.GaugeFloat64:
			optronObj[name] = metric.Value()
		case metrics.Healthcheck:
			metric.Check()
			optronObj[name] = metric.Error()
		case metrics.Histogram:
			h := metric.Snapshot()
			optronObj[name+"_avg"] = h.Mean()
		case metrics.Meter:
			m := metric.Snapshot()
			optronObj[name+"_1MR"] = m.Rate1()
			optronObj[name+"_5MR"] = m.Rate5()
			optronObj[name+"_15MR"] = m.Rate15()
			optronObj[name+"_avg"] = m.RateMean()
		case metrics.Timer:
			scale := float64(time.Second)
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.80, 0.90, 0.95, 0.99})
			optronObj[name+"_avg"] = ps[0] / scale
			optronObj[name+"_80"] = ps[1] / scale
			optronObj[name+"_90"] = ps[2] / scale
			optronObj[name+"_95"] = ps[3] / scale
			optronObj[name+"_99"] = ps[4] / scale
		}

		this.builder.append(optronObj)
	})

	content := this.builder.flush()
	for _, data := range content {
		dataToPost, err := json.Marshal(data)
		if err != nil {
			this.l.Printf("ERROR: optron: marshal: %#v %v", data, err)
			return
		}

		dataToPost = append(dataToPost, []byte("\r\n")...)
		_, err = this.conn.Write(dataToPost)
		if err != nil {
			this.l.Printf("Warn: optron: send: %v", err)
			this.connect()
		}
	}
}

func New(name, configUri string, interval time.Duration, l Logger) (*Optron, error) {
	o := &Optron{
		name:     name,
		interval: interval,
		l:        l,
	}
	return o, o.init(configUri)
}

func NewForGame(game, name, configUri string, interval time.Duration, l Logger) (*Optron, error) {
	o := &Optron{
		game:     game,
		name:     name,
		interval: interval,
		l:        l,
	}
	return o, o.init(configUri)
}
