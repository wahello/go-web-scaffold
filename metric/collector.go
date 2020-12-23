package metric

import (
	"io"
	"net/http"
	"time"

	"github.com/segmentio/stats/v4"
	"github.com/segmentio/stats/v4/procstats"
	"github.com/segmentio/stats/v4/prometheus"
)

// listeningAddr address where metric server listening
const listeningAddr = ":2000"

// Collector collects and reports metrics, use factory functions to create
type Collector struct {
	engine                *stats.Engine
	handler               *prometheus.Handler
	processStatsCollector io.Closer
}

// NewPrometheusCollector creates a Prometheus-based Collector
func NewPrometheusCollector(prefix string, collectProcessStats bool, tag ...stats.Tag) *Collector {
	handler := &prometheus.Handler{
		TrimPrefix:    "",
		MetricTimeout: 1 * time.Minute,
		Buckets:       stats.HistogramBuckets{},
	}

	engine := stats.NewEngine(prefix, handler)
	if len(tag) > 0 {
		engine = engine.WithTags(tag...)
	}

	var processStatsCollector io.Closer
	if collectProcessStats {
		processStatsCollector = procstats.StartCollector(procstats.NewGoMetricsWith(engine))
	}

	return &Collector{
		engine:                engine,
		handler:               handler,
		processStatsCollector: processStatsCollector,
	}
}

// NewNopCollector creates a dummy Collector
func NewNopCollector() *Collector {
	return &Collector{}
}

// Flush flushes any buffered data
func (c *Collector) Flush() {
	if c.engine != nil {
		c.engine.Flush()
	}
}

// Close should be called before program exiting
func (c *Collector) Close() {
	if c.processStatsCollector != nil {
		_ = c.processStatsCollector.Close()
	}
	c.Flush()
}

// ServeMetrics starts metric server for scape, usually runs in goroutine
//
// service is listening at listeningAddr
func (c *Collector) ServeMetrics() {
	if c.handler == nil {
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", c.handler)

	server := http.Server{
		Addr:              listeningAddr,
		Handler:           mux,
		ReadTimeout:       2 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      2 * time.Second,
	}
	_ = server.ListenAndServe()
}

// Incr increments by one the counter identified by name and tags.
func (c *Collector) Incr(name string, tags ...stats.Tag) {
	if c.engine == nil {
		return
	}
	c.engine.Incr(name, tags...)
}

// Add increments by value the counter identified by name and tags.
func (c *Collector) Add(name string, value interface{}, tags ...stats.Tag) {
	if c.engine == nil {
		return
	}
	c.engine.Add(name, value, tags...)
}

// Observe reports value for the histogram identified by name and tags.
func (c *Collector) Observe(name string, value interface{}, tags ...stats.Tag) {
	if c.engine == nil {
		return
	}
	c.engine.Observe(name, value, tags...)
}

// Set sets to value the gauge identified by name and tags.
func (c *Collector) Set(name string, value interface{}, tags ...stats.Tag) {
	if c.engine == nil {
		return
	}
	c.engine.Set(name, value, tags...)
}

// Report reports metrics use a struct
//
// example: https://github.com/segmentio/stats#migration-to-v4
func (c *Collector) Report(metrics interface{}, tags ...stats.Tag) {
	if c.engine == nil {
		return
	}
	c.engine.Report(metrics, tags...)
}
