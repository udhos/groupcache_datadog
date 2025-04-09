[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/udhos/groupcache_datadog/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/udhos/groupcache_datadog)](https://goreportcard.com/report/github.com/udhos/groupcache_datadog)
[![Go Reference](https://pkg.go.dev/badge/github.com/udhos/groupcache_datadog.svg)](https://pkg.go.dev/github.com/udhos/groupcache_datadog)

# groupcache_datadog

[groupcache_datadog](https://github.com/udhos/groupcache_datadog) exports [groupcache](https://github.com/modernprogram/groupcache) metrics to Datadog.

# Usage

See example [./examples/groupcache-datadog-modernprogram/main.go](./examples/groupcache-datadog-modernprogram/main.go).

# Build example

```bash
git clone https://github.com/udhos/groupcache_datadog
cd groupcache_datadog
go install ./...
```

# Running example

By default the example `groupcache-datadog-modernprogram` sends metrics to `localhost:8125`.

```bash
$ groupcache-datadog-modernprogram
2025/04/09 00:50:34 groupcache ttl: 30s
2025/04/09 00:50:34 groupcache my URL: http://127.0.0.1:5000
2025/04/09 00:50:34 INFO DD_AGENT_HOST=[] using DD_AGENT_HOST=localhost default=localhost
2025/04/09 00:50:34 groupcache server: listening on :5000
2025/04/09 00:50:34 INFO DD_AGENT_PORT=[] using DD_AGENT_PORT=8125 default=8125
2025/04/09 00:50:34 INFO DD_SERVICE=[] using DD_SERVICE=service-unknown default=service-unknown
2025/04/09 00:50:34 INFO DD_TAGS=[] using DD_TAGS= default=
2025/04/09 00:50:34 INFO NewDatadogClient host=localhost:8125 namespace=groupcache service=service-unknown tags=[service:service-unknown]
2025/04/09 00:50:34 getter: loading: key:/etc/passwd, ttl:30s
2025/04/09 00:50:34 INFO cache answer: bytes=2943 elapsed=50.548208ms, sleeping 5s
```

# Datadog Count is a Delta

## Definition

The COUNT metric submission type represents the total number of event occurrences in one time interval. A COUNT can be used to track the total number of connections made to a database or the total number of requests to an endpoint. This number of events can accumulate or decrease over time—it is not monotonically increasing.

## Example

Suppose you are submitting a COUNT metric, notifications.sent, from a single host running the Datadog Agent. This host emits the following values in a flush time interval: [1,1,1,2,2,2,3,3].

The Agent adds all of the values received in one time interval. Then, it submits the total number, in this case 15, as the COUNT metric’s value.

Source: https://docs.datadoghq.com/metrics/types/?tab=count#definition
