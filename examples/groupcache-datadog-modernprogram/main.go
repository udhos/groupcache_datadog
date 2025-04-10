// Package main implements the example.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/modernprogram/groupcache/v2"
	"github.com/udhos/dogstatsdclient/dogstatsdclient"
	"github.com/udhos/groupcache_datadog/exporter"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/modernprogram"
)

func main() {

	var mockStatsd bool
	flag.BoolVar(&mockStatsd, "mockStatsd", false, "mock statsd")
	flag.Parse()
	slog.Info("flag", "mockStatds", mockStatsd)

	cache := startGroupcache()

	//
	// metrics exporter
	//

	var client exporter.ClientInterface

	if mockStatsd {
		client = &statsdMock{}
	} else {
		c, errClient := dogstatsdclient.New(dogstatsdclient.Options{
			Namespace: "groupcache",
			Debug:     true,
		})
		if errClient != nil {
			slog.Error(errClient.Error())
			os.Exit(1)
		}
		client = c
	}

	exporter := exporter.New(exporter.Options{
		Client:         client,
		Groups:         []groupcache_exporter.GroupStatistics{modernprogram.New(cache)},
		ExportInterval: 20 * time.Second,
	})
	defer exporter.Close()

	//
	// query cache periodically
	//

	const interval = 5 * time.Second

	for i := 0; ; i++ {
		query(cache, "/etc/passwd")             // repeat key
		query(cache, fmt.Sprintf("fake-%d", i)) // always miss, and gets evicted
		time.Sleep(interval)
	}
}

func query(cache *groupcache.Group, key string) {
	begin := time.Now()
	var dst []byte
	cache.Get(context.TODO(), key, groupcache.AllocatingByteSliceSink(&dst), nil)
	elap := time.Since(begin)

	slog.Info(fmt.Sprintf("cache answer: bytes=%d elapsed=%v",
		len(dst), elap))
}

type statsdMock struct {
}

// Gauge measures the value of a metric at a particular time.
func (s *statsdMock) Gauge(name string, value float64, tags []string, rate float64) error {
	slog.Info(
		"statsdMock.Gauge",
		"name", name,
		"value", value,
		"tags", tags,
		"rate", rate,
	)
	return nil
}

// Count tracks how many times something happened per second.
func (s *statsdMock) Count(name string, value int64, tags []string, rate float64) error {
	slog.Info(
		"statsdMock.Count",
		"name", name,
		"value", value,
		"tags", tags,
		"rate", rate,
	)
	return nil
}

// Close the client connection.
func (s *statsdMock) Close() error {
	return nil
}
