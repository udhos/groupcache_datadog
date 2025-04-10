// Package mock mocks dogstatsdclient.
package mock

import "log/slog"

// StatsdMock mocks dogstatsd client.
type StatsdMock struct {
}

// Gauge measures the value of a metric at a particular time.
func (s *StatsdMock) Gauge(name string, value float64, tags []string, rate float64) error {
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
func (s *StatsdMock) Count(name string, value int64, tags []string, rate float64) error {
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
func (s *StatsdMock) Close() error {
	return nil
}
