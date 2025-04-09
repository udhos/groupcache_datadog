// Package exporter implements datadog exporter for groupcache.
package exporter

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/udhos/groupcache_exporter"
)

// Options define exporter options.
type Options struct {
	// Groups is list of groupcache groups to export metrics.
	//
	// This is one example about how to create a group suitable to Groups:
	// import "github.com/udhos/groupcache_exporter/groupcache/modernprogram"
	// cache := ... // create groupcache group
	// groupStat := modernprogram.New(cache)
	//
	Groups []groupcache_exporter.GroupStatistics

	// Client interface is implemented by *statsd.Client.
	Client statsd.ClientInterface

	// SampleRate defaults to 1.
	SampleRate float64

	// ExportInterval defaults to 1 minute.
	ExportInterval time.Duration
}

// Exporter exports stats.
type Exporter struct {
	options Options
	done    chan struct{}
}

// New creates an exporter.
func New(options Options) *Exporter {
	if options.SampleRate == 0 {
		options.SampleRate = 1
	}

	if options.ExportInterval == 0 {
		options.ExportInterval = time.Minute
	}

	e := &Exporter{
		options: options,
		done:    make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-e.done:
				return
			default:
				e.exportOnce()
			}
			time.Sleep(options.ExportInterval)
		}
	}()

	return e
}

// exportOnce all metrics once.
func (e *Exporter) exportOnce() {
	for _, g := range e.options.Groups {
		e.exportGroup(g)
	}
}

// Close finishes the exporter.
func (e *Exporter) Close() error {
	close(e.done)
	return e.options.Client.Close()
}

func (e *Exporter) exportGroup(g groupcache_exporter.GroupStatistics) {
	groupName := g.Name()
	tags := []string{fmt.Sprintf("group:%s", groupName)}

	e.exportCount("gets", g.Gets(), tags)
	e.exportCount("hits", g.CacheHits(), tags)
	e.exportGauge("get_from_peers_latency_slowest_milliseconds", float64(g.GetFromPeersLatencyLower()), tags)
	e.exportCount("peer_loads", g.PeerLoads(), tags)
	e.exportCount("peer_errors", g.PeerErrors(), tags)
	e.exportCount("loads", g.Loads(), tags)
	e.exportCount("loads_deduped", g.LoadsDeduped(), tags)
	e.exportCount("local_load", g.LocalLoads(), tags)
	e.exportCount("local_load_errs", g.LocalLoadErrs(), tags)
	e.exportCount("server_requests", g.ServerRequests(), tags)
	e.exportCount("crosstalk_refusals", g.CrosstalkRefusals(), tags)

	tagsMain := append(tags, "type:main")
	e.exportGauge("cache_items", float64(g.MainCacheItems()), tagsMain)
	e.exportGauge("cache_bytes", float64(g.MainCacheBytes()), tagsMain)
	e.exportCount("cache_gets", g.MainCacheGets(), tagsMain)
	e.exportCount("cache_hits", g.MainCacheHits(), tagsMain)
	e.exportCount("cache_evictions", g.MainCacheEvictions(), tagsMain)
	e.exportCount("cache_evictions_nonexpired", g.MainCacheEvictionsNonExpired(), tagsMain)

	tagsHot := append(tags, "type:hot")
	e.exportGauge("cache_items", float64(g.HotCacheItems()), tagsHot)
	e.exportGauge("cache_bytes", float64(g.HotCacheBytes()), tagsHot)
	e.exportCount("cache_gets", g.MainCacheGets(), tagsHot)
	e.exportCount("cache_hits", g.MainCacheHits(), tagsHot)
	e.exportCount("cache_evictions", g.MainCacheEvictions(), tagsHot)
	e.exportCount("cache_evictions_nonexpired", g.MainCacheEvictionsNonExpired(), tagsHot)
}

func (e *Exporter) exportCount(metricName string, value int64, tags []string) {
	if err := e.options.Client.Count(metricName, value, tags, e.options.SampleRate); err != nil {
		slog.Error(fmt.Sprintf("exportCount: error: %v", err))
	}
}

func (e *Exporter) exportGauge(metricName string, value float64, tags []string) {
	if err := e.options.Client.Gauge(metricName, value, tags, e.options.SampleRate); err != nil {
		slog.Error(fmt.Sprintf("exportGauge: error: %v", err))
	}
}

// DatadogClientOptions define options for datadog client.
type DatadogClientOptions struct {
	// Host defaults to env var DD_AGENT_HOST. Undefined DD_AGENT_HOST defaults to localhost.
	Host string

	// Port defaults to env var DD_AGENT_PORT. Undefined DD_AGENT_PORT defaults to 8125.
	Port string

	// Namespace defaults to groupcache.
	Namespace string

	// Service is used to define default Tags. If undefined, defaults to DD_SERVICE.
	Service string

	// Tags defaults to env var DD_TAGS.
	Tags []string

	Debug bool
}

// NewDatadogClient creates datadog client.
func NewDatadogClient(options DatadogClientOptions) (*statsd.Client, error) {

	const me = "NewDatadogClient"

	if options.Host == "" {
		options.Host = envString("DD_AGENT_HOST", "localhost")
	}

	if options.Port == "" {
		options.Port = envString("DD_AGENT_PORT", "8125")
	}

	if options.Namespace == "" {
		options.Namespace = "groupcache"
	}

	if options.Service == "" {
		options.Service = envString("DD_SERVICE", "service-unknown")
	}

	if len(options.Tags) == 0 {
		options.Tags = strings.Fields(envString("DD_TAGS", ""))
	}

	// add service to tags
	options.Tags = append(options.Tags, fmt.Sprintf("service:%s", options.Service))

	slices.Sort(options.Tags)
	options.Tags = slices.Compact(options.Tags)

	host := fmt.Sprintf("%s:%s", options.Host, options.Port)

	if options.Debug {
		slog.Info(
			me,
			"host", host,
			"namespace", options.Namespace,
			"service", options.Service,
			"tags", options.Tags,
		)
	}

	c, err := statsd.New(host,
		statsd.WithNamespace(options.Namespace),
		statsd.WithTags(options.Tags))

	return c, err
}

// envString extracts string from env var.
// It returns the provided defaultValue if the env var is empty.
// The string returned is also recorded in logs.
func envString(name string, defaultValue string) string {
	str := os.Getenv(name)
	if str != "" {
		slog.Info(fmt.Sprintf("%s=[%s] using %s=%s default=%s",
			name, str, name, str, defaultValue))
		return str
	}
	slog.Info(fmt.Sprintf("%s=[%s] using %s=%s default=%s",
		name, str, name, defaultValue, defaultValue))
	return defaultValue
}
