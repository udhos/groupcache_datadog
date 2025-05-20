// Package exporter implements datadog exporter for groupcache.
package exporter

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/udhos/groupcache_exporter"
)

// Options define exporter options.
type Options struct {
	// ListGroups function must provide current list of groupcache groups.
	ListGroups func() []groupcache_exporter.GroupStatistics

	// Client interface is implemented by *statsd.Client.
	Client ClientInterface

	// SampleRate defaults to 1.
	SampleRate float64

	// ExportInterval defaults to 1 minute.
	ExportInterval time.Duration

	// HostnameTagKey defaults to "pod_name".
	HostnameTagKey string

	// DisableHostnameTag prevents adding hostname tag $HostnameTagKey:$hostname.
	DisableHostnameTag bool

	// Debug enables debugging logs.
	Debug bool
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
	previousStats map[string]groupcache_exporter.Stats
	hostname      string
}

// New creates an exporter.
func New(options Options) *Exporter {

	if options.SampleRate == 0 {
		options.SampleRate = 1
	}

	if options.ExportInterval == 0 {
		options.ExportInterval = time.Minute
	}

	var hostname string

	if !options.DisableHostnameTag {
		if options.HostnameTagKey == "" {
			options.HostnameTagKey = "pod_name"
		}
		h, err := os.Hostname()
		if err != nil {
			slog.Error(fmt.Sprintf("groupcache_datadog: exporter.New: hostname error: %v", err))
		}
		hostname = h
	}

	e := &Exporter{
		options:       options,
		done:          make(chan struct{}),
		hostname:      hostname,
		previousStats: map[string]groupcache_exporter.Stats{},
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
	for _, g := range e.options.ListGroups() {
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

	if e.hostname != "" {
		tags = append(tags, fmt.Sprintf("%s:%s",
			e.options.HostnameTagKey, e.hostname))
	}

	previousStats := e.previousStats[groupName]
	previousGroup := previousStats.Group

	stats := g.Collect()

	if e.options.Debug {
		slog.Info("exportGroup",
			"group", groupName,
			"stats", stats,
		)
	}

	group := stats.Group

	//
	// datadog counter is delta
	//
	delta := groupcache_exporter.GetCacheDelta(previousGroup, group)

	e.exportCount("gets", delta.Gets, tags)
	e.exportCount("hits", delta.Hits, tags)
	e.exportGauge("get_from_peers_latency_slowest_milliseconds", float64(group.GaugeGetFromPeersLatencyLower), tags)
	e.exportCount("peer_loads", delta.PeerLoads, tags)
	e.exportCount("peer_errors", delta.PeerErrors, tags)
	e.exportCount("loads", delta.Loads, tags)
	e.exportCount("loads_deduped", delta.LoadsDeduped, tags)
	e.exportCount("local_load", delta.LocalLoads, tags)
	e.exportCount("local_load_errs", delta.LocalLoadsErrs, tags)
	e.exportCount("server_requests", delta.ServerRequests, tags)
	e.exportCount("crosstalk_refusals", delta.CrosstalkRefusals, tags)

	e.exportGroupType(previousStats.Main, stats.Main, tags, "type:main")
	e.exportGroupType(previousStats.Hot, stats.Hot, tags, "type:hot")

	e.previousStats[groupName] = stats // save for next collection
}

func (e *Exporter) exportGroupType(prev,
	curr groupcache_exporter.CacheTypeStats, tags []string,
	cacheType string) {

	t := append(tags, cacheType)

	//
	// datadog counter is delta
	//
	delta := groupcache_exporter.GetCacheTypeDelta(prev, curr)

	e.exportGauge("cache_items", float64(curr.GaugeCacheItems), t)
	e.exportGauge("cache_bytes", float64(curr.GaugeCacheBytes), t)
	e.exportCount("cache_gets", delta.Gets, t)
	e.exportCount("cache_hits", delta.Hits, t)
	e.exportCount("cache_evictions", delta.Evictions, t)
	e.exportCount("cache_evictions_nonexpired", delta.EvictionsNonExpired, t)
}

func (e *Exporter) debugMetric(metricName string, value any, tags []string) {
	if e.options.Debug {
		slog.Info("groupcache_datadog debugMetric",
			"metric", metricName,
			"value", value,
			"tags", tags,
			"sample_rate", e.options.SampleRate,
		)
	}
}

func (e *Exporter) exportCount(metricName string, value int64, tags []string) {
	e.debugMetric(metricName, value, tags)
	if err := e.options.Client.Count(metricName, value, tags, e.options.SampleRate); err != nil {
		slog.Error(fmt.Sprintf("groupcache_datadog exportCount: error: %v", err))
	}
}

func (e *Exporter) exportGauge(metricName string, value float64, tags []string) {
	e.debugMetric(metricName, value, tags)
	if err := e.options.Client.Gauge(metricName, value, tags, e.options.SampleRate); err != nil {
		slog.Error(fmt.Sprintf("groupcache_datadog exportGauge: error: %v", err))
	}
}
