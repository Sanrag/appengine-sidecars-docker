package dockerstats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"

	mpb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type fakeDocker struct {
	client.Client
}

func (d *fakeDocker) ContainerList(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
	return []types.Container{
		{
			ID:    "id1",
			Names: []string{"name1a", "name1b"},
		},
		{
			ID:    "id2",
			Names: []string{},
		},
		{
			ID:    "id3",
			Names: []string{"name3"},
		},
	}, nil
}

func (d *fakeDocker) ContainerStats(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
	s1 := types.StatsJSON{
		Stats: types.Stats{
			MemoryStats: types.MemoryStats{
				Usage: 33,
				Limit: 66,
			},
		},
		Networks: map[string]types.NetworkStats{
			"eth0": {
				RxBytes: 111,
				TxBytes: 222,
			},
		},
	}
	s2 := types.StatsJSON{
		Stats: types.Stats{
			MemoryStats: types.MemoryStats{
				Usage: 44,
				Limit: 88,
			},
		},
		Networks: map[string]types.NetworkStats{
			"eth0": {
				RxBytes: 333,
				TxBytes: 444,
			},
			"eth1": {
				RxBytes: 222,
				TxBytes: 333,
			},
		},
	}
	s3 := types.StatsJSON{}

	var stats types.StatsJSON
	var err error
	switch id {
	case "id1":
		stats = s1
	case "id2":
		stats = s2
	case "id3":
		stats = s3
		err = fmt.Errorf("manual failure")
	}

	b, err2 := json.Marshal(stats)
	if err2 != nil {
		return types.ContainerStats{}, fmt.Errorf("failed to marshal JSON: %v", err2)
	}

	return types.ContainerStats{
		Body: ioutil.NopCloser(bytes.NewReader(b)),
	}, err
}

func (d *fakeDocker) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	var c types.ContainerJSON
	var err error

	switch id {
	case "id1":
		c = types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				RestartCount: 3,
				State: &types.ContainerState{
					StartedAt: "2019-12-31T12:00:00.000000000Z",
				},
			},
		}
	case "id2":
		c = types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				RestartCount: 5,
				State: &types.ContainerState{
					StartedAt: "2019-12-31T00:00:00.000000000Z",
				},
			},
		}
	case "id3":
		c = types.ContainerJSON{}
		err = fmt.Errorf("manual error")
	}

	return c, err
}

// fakeMetricConsumer extends consumer.MetricsConsumer.
type fakeMetricsConsumer struct {
	metrics pdata.Metrics
}

func (c *fakeMetricsConsumer) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	c.metrics = md
	return nil
}

func fakeNow() time.Time {
	t, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	return t
}

func TestScraperExport(t *testing.T) {
	c := &fakeMetricsConsumer{}
	s := &scraper{
		startTime:      fakeNow(),
		metricConsumer: c,
		docker:         &fakeDocker{},
		scrapeInterval: 10 * time.Second,
		now:            fakeNow,
	}

	s.export()

	data := pdatautil.MetricsToMetricsData(c.metrics)[0]
	verifyContainerMetricValue(t, data, "container/memory/usage", "name1a", 33)
	verifyContainerMetricValue(t, data, "container/memory/limit", "name1a", 66)
	verifyContainerMetricValue(t, data, "container/network/received_bytes", "name1a", 111)
	verifyContainerMetricValue(t, data, "container/network/sent_bytes", "name1a", 222)
	verifyContainerMetricValue(t, data, "container/uptime", "name1a", 43200)
	verifyContainerMetricValue(t, data, "container/restart_count", "name1a", 3)
	verifyContainerMetricValue(t, data, "container/memory/usage", "id2", 44)
	verifyContainerMetricValue(t, data, "container/memory/limit", "id2", 88)
	verifyContainerMetricValue(t, data, "container/network/received_bytes", "id2", 555)
	verifyContainerMetricValue(t, data, "container/network/sent_bytes", "id2", 777)
	verifyContainerMetricValue(t, data, "container/uptime", "id2", 86400)
	verifyContainerMetricValue(t, data, "container/restart_count", "id2", 5)
	verifyContainerMetricAbsent(t, data, "container/memory/usage", "name3")
	verifyContainerMetricAbsent(t, data, "container/memory/limit", "name3")
	verifyContainerMetricAbsent(t, data, "container/network/received_bytes", "name3")
	verifyContainerMetricAbsent(t, data, "container/network/sent_bytes", "name3")
	verifyContainerMetricAbsent(t, data, "container/uptime", "name3")
	verifyContainerMetricAbsent(t, data, "container/restart_count", "name3")
}

func verifyContainerMetricValue(t *testing.T, data consumerdata.MetricsData, name, label string, value int64) {
	var metric *mpb.Metric
	for _, m := range data.Metrics {
		if m.MetricDescriptor.Name == name && m.Timeseries[0].LabelValues[0].Value == label {
			metric = m
		}
	}
	if metric == nil {
		t.Errorf("Unable to find metric %q", name)
		return
	}
	assert.Equal(t, value, metric.Timeseries[0].Points[0].GetInt64Value())
}

func verifyContainerMetricAbsent(t *testing.T, data consumerdata.MetricsData, name, label string) {
	for _, m := range data.Metrics {
		if m.MetricDescriptor.Name == name && m.Timeseries[0].LabelValues[0].Value == label {
			t.Errorf("Expected metric %s{container_name=%s} to be absent, found metric: %v", name, label, m)
			break
		}
	}
}

type alwaysFailDocker struct {
	client.Client
}

func (d *alwaysFailDocker) ContainerList(_ context.Context, _ types.ContainerListOptions) ([]types.Container, error) {
	return []types.Container{}, fmt.Errorf("always fail")
}

func TestScraperContinuesOnError(t *testing.T) {
	s := &scraper{
		now:            fakeNow,
		docker:         &alwaysFailDocker{},
		scrapeInterval: 1 * time.Second,
		done:           make(chan bool),
	}
	s.start()
	time.Sleep(6 * time.Second)
	s.stop()
	assert.GreaterOrEqual(t, s.scrapeCount, uint64(5))
}
