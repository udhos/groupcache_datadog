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
	Client ClientInterface

	// SampleRate defaults to 1.
	SampleRate float64

	// ExportInterval defaults to 1 minute.
	ExportInterval time.Duration
}

// ClientInterface is implemented by *statsd.Client.
// Simplified version of statsd.ClientInterface.
type ClientInterface interface {
	// Gauge measures the value of a metric at a particular time.
	Gauge(name string, value float64, tags []string, rate float64) error

	// Count tracks how many times something happened per second.
	Count(name string, value int64, tags []string, rate float64) error

	// Close the client connection.
	Close() error
}

// Exporter exports stats.
type Exporter struct {
	options       Options
	done          chan struct{}
	previousStats groupcache_exporter.Stats
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

	previousGroup := e.previousStats.Group

	stats := g.Collect()
	group := stats.Group

	//
	// datadog counter is delta
	//
	deltaGets := group.CounterGets - previousGroup.CounterGets
	deltaHits := group.CounterHits - previousGroup.CounterHits
	deltaPeerLoads := group.CounterPeerLoads - previousGroup.CounterPeerLoads
	deltaPeerErrors := group.CounterPeerErrors - previousGroup.CounterPeerErrors
	deltaLoads := group.CounterLoads - previousGroup.CounterLoads
	deltaLoadsDeduped := group.CounterLoadsDeduped - previousGroup.CounterLoadsDeduped
	deltaLocalLoads := group.CounterLocalLoads - previousGroup.CounterLocalLoads
	deltaLocalLoadsErrs := group.CounterLocalLoadsErrs - previousGroup.CounterLocalLoadsErrs
	deltaServerRequests := group.CounterServerRequests - previousGroup.CounterServerRequests
	deltaCrosstalkRefusals := group.CounterCrosstalkRefusals - previousGroup.CounterCrosstalkRefusals

	e.exportCount("gets", deltaGets, tags)
	e.exportCount("hits", deltaHits, tags)
	e.exportGauge("get_from_peers_latency_slowest_milliseconds", float64(group.GaugeGetFromPeersLatencyLower), tags)
	e.exportCount("peer_loads", deltaPeerLoads, tags)
	e.exportCount("peer_errors", deltaPeerErrors, tags)
	e.exportCount("loads", deltaLoads, tags)
	e.exportCount("loads_deduped", deltaLoadsDeduped, tags)
	e.exportCount("local_load", deltaLocalLoads, tags)
	e.exportCount("local_load_errs", deltaLocalLoadsErrs, tags)
	e.exportCount("server_requests", deltaServerRequests, tags)
	e.exportCount("crosstalk_refusals", deltaCrosstalkRefusals, tags)

	e.exportGroupType(e.previousStats.Main, stats.Main, tags, "type:main")
	e.exportGroupType(e.previousStats.Hot, stats.Hot, tags, "type:hot")

	e.previousStats = stats // save for next collection
}

func (e *Exporter) exportGroupType(prev,
	curr groupcache_exporter.CacheTypeStats, tags []string,
	cacheType string) {

	t := append(tags, cacheType)

	//
	// datadog counter is delta
	//
	delta := getCacheDelta(prev, curr)

	e.exportGauge("cache_items", float64(curr.GaugeCacheItems), t)
	e.exportGauge("cache_bytes", float64(curr.GaugeCacheBytes), t)
	e.exportCount("cache_gets", delta.gets, t)
	e.exportCount("cache_hits", delta.hits, t)
	e.exportCount("cache_evictions", delta.evictions, t)
	e.exportCount("cache_evictions_nonexpired", delta.evictionsNonExpired, t)
}

type cacheDelta struct {
	gets                int64
	hits                int64
	evictions           int64
	evictionsNonExpired int64
}

func getCacheDelta(prev, curr groupcache_exporter.CacheTypeStats) cacheDelta {
	return cacheDelta{
		gets:                curr.CounterCacheGets - prev.CounterCacheGets,
		hits:                curr.CounterCacheHits - prev.CounterCacheHits,
		evictions:           curr.CounterCacheEvictions - prev.CounterCacheEvictions,
		evictionsNonExpired: curr.CounterCacheEvictionsNonExpired - prev.CounterCacheEvictionsNonExpired,
	}
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
