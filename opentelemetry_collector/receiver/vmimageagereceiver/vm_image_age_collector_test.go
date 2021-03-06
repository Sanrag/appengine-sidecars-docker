package vmimageagereceiver

import (
	"context"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
)

func TestCalculateImageAge(t *testing.T) {
	now := time.Date(2020, time.January, 29, 0, 0, 0, 0, time.UTC)
	buildTime := time.Date(2019, time.December, 31, 0, 0, 0, 0, time.UTC)

	age, err := calculateImageAge(buildTime, now)
	assert.Nil(t, err)
	assert.Equal(t, float64(29), age)
}

func TestCalculateImageAgeWithNegativeAge(t *testing.T) {
	now := time.Date(2020, time.January, 29, 12, 30, 0, 0, time.UTC)
	buildTime := time.Date(2020, time.January, 31, 12, 30, 0, 0, time.UTC)

	_, err := calculateImageAge(buildTime, now)
	assert.Error(t, err)
}

func TestCalculateImageAgeWith0Age(t *testing.T) {
	now := time.Date(2020, time.January, 29, 12, 0, 0, 0, time.UTC)
	buildTime := time.Date(2020, time.January, 29, 6, 0, 0, 0, time.UTC)

	age, err := calculateImageAge(buildTime, now)
	assert.Nil(t, err)
	assert.Equal(t, float64(0.25), age)
}

func TestParseBuildDate(t *testing.T) {
	collector := NewVMImageAgeCollector(0, "2006-01-02T15:04:05+00:00", "test_image_name", nil)
	collector.parseBuildDate()
	assert.False(t, collector.buildDateError)
	diff := collector.parsedBuildDate.Sub(time.Date(2006, time.January, 2, 15, 4, 5, 0, time.FixedZone("", 0)))
	assert.Equal(t, diff, time.Second*0)
}

func TestParseBuildDateError(t *testing.T) {
	collector := NewVMImageAgeCollector(0, "misformated_date", "test_image_name", nil)
	collector.parseBuildDate()
	assert.True(t, collector.buildDateError)
}

type fakeConsumer struct {
	storage *metricsStore
}

type metricsStore struct {
	metrics pdata.Metrics
}

func (s *metricsStore) storeMetric(toStore pdata.Metrics) {
	s.metrics = toStore
}

func (consumer fakeConsumer) ConsumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	consumer.storage.storeMetric(metrics)
	return nil
}

func TestScrapeAndExport(t *testing.T) {
	consumer := fakeConsumer{storage: &metricsStore{}}
	collector := NewVMImageAgeCollector(0, "2006-01-02T15:04:05+00:00", "test_image_name", consumer)
	collector.setupCollection()
	collector.scrapeAndExport()

	expectedMetricDescriptor := &metricspb.MetricDescriptor{
		Name:        "vm_image_ages",
		Description: "The VM image age for the VM instance",
		Unit:        "Days",
		Type:        metricspb.MetricDescriptor_GAUGE_DISTRIBUTION,
		LabelKeys: []*metricspb.LabelKey{{
			Key:         "vm_image_name",
			Description: "The name of the VM image",
		}},
	}

	// TODO: Rewrite tests to directly use pdata.Metrics instead of converting back to consumerdata.MetricsData.
	cdMetrics := pdatautil.MetricsToMetricsData(consumer.storage.metrics)[0]
	if assert.Len(t, cdMetrics.Metrics, 1) {

		actualMetric := cdMetrics.Metrics[0]
		assert.Equal(t, expectedMetricDescriptor, actualMetric.MetricDescriptor)

		if assert.Len(t, actualMetric.Timeseries, 1) {
			expectedLabel := []*metricspb.LabelValue{{Value: "test_image_name", HasValue: true}}
			timeseries := actualMetric.Timeseries[0]
			assert.Equal(t, expectedLabel, timeseries.LabelValues)

			if assert.Len(t, timeseries.Points, 1) {
				point := timeseries.Points[0].GetDistributionValue()
				assert.Equal(t, int64(1), point.Count)
				assert.Equal(t, float64(0), point.SumOfSquaredDeviation)

				expectedBuckets := []*metricspb.DistributionValue_Bucket{
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 0},
					{Count: 1},
				}

				assert.Equal(t, expectedBuckets, point.GetBuckets())

				assert.Equal(t, []float64{1, 2, 4, 8, 16, 32, 64, 128, 256}, point.GetBucketOptions().GetExplicit().Bounds)
			}
		}
	}
}

func TestScrapeAndExportWithError(t *testing.T) {
	consumer := fakeConsumer{storage: &metricsStore{}}
	collector := NewVMImageAgeCollector(0, "", "test_image_name", consumer)
	collector.setupCollection()
	collector.scrapeAndExport()

	expectedMetricDescriptor := &metricspb.MetricDescriptor{
		Name:        "vm_image_ages_error",
		Description: "The current number of VM instances with errors exporting the VM image age.",
		Unit:        "Count",
		Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		LabelKeys: []*metricspb.LabelKey{{
			Key:         "vm_image_name",
			Description: "The name of the VM image",
		}},
	}

	// TODO: Rewrite tests to directly use pdata.Metrics instead of converting back to consumerdata.MetricsData.
	cdMetrics := pdatautil.MetricsToMetricsData(consumer.storage.metrics)[0]
	if assert.Len(t, cdMetrics.Metrics, 1) {

		actualMetric := cdMetrics.Metrics[0]
		assert.Equal(t, expectedMetricDescriptor, actualMetric.MetricDescriptor)

		if assert.Len(t, actualMetric.Timeseries, 1) {
			expectedLabel := []*metricspb.LabelValue{{Value: "test_image_name", HasValue: true}}
			timeseries := actualMetric.Timeseries[0]
			assert.Equal(t, expectedLabel, timeseries.LabelValues)

			if assert.Len(t, timeseries.Points, 1) {
				assert.Equal(t, int64(1), timeseries.Points[0].GetInt64Value())
			}
		}
	}
}
