package dockerstats

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines the configuration for dockerstats receiver.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	// ScrapeInterval controls how often docker stats are scraped from docker API.
	ScrapeInterval time.Duration `mapstructure:"scrape_interval"`
}
