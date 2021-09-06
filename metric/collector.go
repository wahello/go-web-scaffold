package metric

import (
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/segmentio/stats/v4"
	"github.com/segmentio/stats/v4/procstats"
	"github.com/segmentio/stats/v4/prometheus"
)

// listeningAddr address where metric server listening
const listeningAddr = ":2000"

const sep = "."

// DefaultSecondBuckets good for API timing
var DefaultSecondBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// DefaultByteBuckets good for data size
var DefaultByteBuckets = []float64{1 << 10, 5 << 10, 10 << 10, 25 << 10, 50 << 10, 100 << 10, 250 << 10, 500 << 10, 1 << 20, 2 << 20, 5 << 20}

// Collector use factory functions to create
type Collector struct {
	engine                *stats.Engine
	handler               *prometheus.Handler
	processStatsCollector io.Closer
}

// HistogramBuckets no need to provide +Inf explicitly
type HistogramBuckets map[string][]float64

// NewPrometheusCollector creates a Collector based on Promethues.
func NewPrometheusCollector(prefix string, buckets HistogramBuckets, collectProcessStats bool, tag ...stats.Tag) *Collector {
	handler := &prometheus.Handler{
		TrimPrefix:    "",
		MetricTimeout: 15 * time.Minute,
		Buckets:       newHistogramBuckets(prefix, buckets),
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

func newHistogramBuckets(prefix string, buckets HistogramBuckets) (product stats.HistogramBuckets) {
	if len(buckets) == 0 {
		product = stats.HistogramBuckets{}
		return
	}

	product = make(stats.HistogramBuckets, len(buckets))
	for k, v := range buckets {
		values := make([]stats.Value, 0, len(v)+1)
		for _, item := range v {
			values = append(values, stats.ValueOf(item))
		}
		values = append(values, stats.ValueOf(math.Inf(1)))
		product[newStatsKey(prefix, k)] = values
	}

	return
}

func newStatsKey(prefix, name string) (key stats.Key) {
	var word = name
	if prefix != "" {
		word = prefix + sep + word
	}

	sepIdx := strings.LastIndex(word, sep)
	if sepIdx == -1 {
		key.Field = word
		return
	}

	key.Measure = word[:sepIdx]
	key.Field = word[sepIdx+1:]

	return
}
