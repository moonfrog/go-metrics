package optron

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/moonfrog/go-metrics"

	"github.com/moonfrog/badger/logs"
	"github.com/moonfrog/badger/utils"
)

type Optron struct {
	config   *ConfigOptronDef
	conn     *net.TCPConn
	interval time.Duration
	working  bool
}

func (this *Optron) init(configUri string) error {
	var err error
	this.config, err = getOptronConfig(configUri)
	if err != nil {
		return fmt.Errorf("optron config: get: %v", err)
	}

	return nil
}

func (this *Optron) Start() {
	for range time.Tick(this.interval) {
		this.send()
	}
}

func (this *Optron) connect() {
	this.working = true
	return
	this.working = false
	conn, err := net.Dial("tcp", this.config.Address)
	if err != nil {
		logs.Warn("optron: connect: %v", err)
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

	optronObj := map[string]interface{}{
		"hostName": utils.GetIpAddress(),
		"id":       "router"}

	metrics.DefaultRegistry.Each(func(name string, m interface{}) {
		switch metric := m.(type) {
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
			ps := t.Percentiles([]float64{0.5, 0.80, 0.95, 0.99, 0.999})
			optronObj[name+"_avg"] = t.Mean() / scale
			optronObj[name+"_80"] = ps[1]
			optronObj[name+"_95"] = ps[1]
			optronObj[name+"_99"] = ps[1]
			optronObj[name+"_99.9"] = ps[1]
		}
	})
	dataToPost, err := json.Marshal(optronObj)
	if err != nil {
		logs.Error("optron: marshal: %#v %v", optronObj, err)
		return
	}

	dataToPost = append(dataToPost, []byte("\r\n")...)
	logs.Info("%#v", string(dataToPost))
	return
	_, err = this.conn.Write(dataToPost)
	if err != nil {
		logs.Warn("optron: send: %v", err)
		this.connect()
	}
}

func New(configUri string, interval time.Duration) (*Optron, error) {
	o := &Optron{
		interval: interval,
	}
	return o, o.init(configUri)
}
