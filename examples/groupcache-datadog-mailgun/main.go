// Package main implements the example.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/mailgun/groupcache/v2"
	"github.com/udhos/dogstatsdclient/dogstatsdclient"
	"github.com/udhos/groupcache_datadog/exporter"
	"github.com/udhos/groupcache_datadog/internal/mock"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/mailgun"
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
		client = &mock.StatsdMock{}
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
		Groups:         []groupcache_exporter.GroupStatistics{mailgun.New(cache)},
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
	cache.Get(context.TODO(), key, groupcache.AllocatingByteSliceSink(&dst))
	elap := time.Since(begin)

	slog.Info(fmt.Sprintf("cache answer: bytes=%d elapsed=%v",
		len(dst), elap))
}
