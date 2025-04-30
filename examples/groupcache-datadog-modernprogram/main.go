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
	"github.com/udhos/groupcache_datadog/internal/mock"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/modernprogram"
)

func main() {

	var mockStatsd bool
	var debugExporter bool
	var debugDogstatsd bool
	flag.BoolVar(&mockStatsd, "mockStatsd", false, "mock statsd")
	flag.BoolVar(&debugExporter, "debugExporter", false, "enable exporter debug")
	flag.BoolVar(&debugDogstatsd, "debugDogstatsd", false, "enable dogstatsd debug")
	flag.Parse()
	slog.Info("flag", "mockStatds", mockStatsd)

	workspace := groupcache.NewWorkspace()

	caches := startGroupcache(workspace)

	//
	// metrics exporter
	//

	var client exporter.ClientInterface

	if mockStatsd {
		client = &mock.StatsdMock{}
	} else {
		c, errClient := dogstatsdclient.New(dogstatsdclient.Options{
			Namespace: "groupcache",
			Debug:     debugDogstatsd,
		})
		if errClient != nil {
			slog.Error(errClient.Error())
			os.Exit(1)
		}
		client = c
	}

	exporter := exporter.New(exporter.Options{
		Client:         client,
		ListGroups:     func() []groupcache_exporter.GroupStatistics { return modernprogram.ListGroups(workspace) },
		ExportInterval: 20 * time.Second,
		Debug:          debugExporter,
	})
	defer exporter.Close()

	//
	// query cache periodically
	//

	const interval = 5 * time.Second

	for i := 0; ; i++ {
		for _, cache := range caches {
			query(cache, "/etc/passwd")             // repeat key
			query(cache, fmt.Sprintf("fake-%d", i)) // always miss, and gets evicted
			time.Sleep(interval)
		}
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
