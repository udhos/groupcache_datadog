// Package main implements the example.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/modernprogram/groupcache/v2"
	"github.com/udhos/groupcache_datadog/exporter"
	"github.com/udhos/groupcache_exporter"
	"github.com/udhos/groupcache_exporter/groupcache/modernprogram"
)

func main() {

	cache := startGroupcache()

	//
	// metrics exporter
	//

	client, errClient := exporter.NewDatadogClient(exporter.DatadogClientOptions{
		Debug: true,
	})
	if errClient != nil {
		slog.Error(errClient.Error())
		os.Exit(1)
	}

	exporter := exporter.New(exporter.Options{
		Client: client,
		Groups: []groupcache_exporter.GroupStatistics{modernprogram.New(cache)},
	})
	defer exporter.Close()

	//
	// query cache periodically
	//

	const interval = 5 * time.Second

	for {
		begin := time.Now()
		var dst []byte
		cache.Get(context.TODO(), "/etc/passwd", groupcache.AllocatingByteSliceSink(&dst), nil)
		elap := time.Since(begin)

		slog.Info(fmt.Sprintf("cache answer: bytes=%d elapsed=%v, sleeping %v",
			len(dst), elap, interval))
		time.Sleep(interval)
	}

}
