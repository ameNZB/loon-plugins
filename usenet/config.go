package usenet

import "strconv"

// Config is the plugins.usenet section of config.yml. The server here seeds the
// servers table on first boot if it's empty; after that the wizard owns it.
// The numeric knobs are DEFAULTS — rows in the plugin's settings table
// (edited on the host's /admin/settings page) override them at job run time
// via withOverrides.
type Config struct {
	Server              ServerConfig `json:"server"`
	RetentionDays       int          `json:"retention_days"`         // keep the last N days (default 3)
	CrawlIntervalMin    int          `json:"crawl_interval_min"`     // crawl cadence (default 15)
	Batch               int          `json:"batch"`                  // article-number span per OVER request (default 3000)
	MaxGroups           int          `json:"max_groups"`             // cap active groups crawled per run (default 20)
	MaxArticlesPerGroup int          `json:"max_articles_per_group"` // cap the first-pass volume so a busy group can't pull millions (default 20000)

	SkipBackfill          bool `json:"skip_backfill"`            // "new articles only" — disable the backfill job
	BackfillBatchesPerRun int  `json:"backfill_batches_per_run"` // cap backward batches per backfill pass, across all groups (default 10)
	BackfillIntervalMin   int  `json:"backfill_interval_min"`    // backfill cadence (default 30)
}

type ServerConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TLS      bool   `json:"tls"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *Config) applyDefaults() {
	if c.RetentionDays <= 0 {
		c.RetentionDays = 3
	}
	if c.CrawlIntervalMin <= 0 {
		c.CrawlIntervalMin = 15
	}
	if c.Batch <= 0 {
		c.Batch = 3000
	}
	if c.MaxGroups <= 0 {
		c.MaxGroups = 20
	}
	if c.MaxArticlesPerGroup <= 0 {
		c.MaxArticlesPerGroup = 20000
	}
	if c.BackfillBatchesPerRun <= 0 {
		c.BackfillBatchesPerRun = 10
	}
	if c.BackfillIntervalMin <= 0 {
		c.BackfillIntervalMin = 30
	}
	if c.Server.Port == 0 {
		c.Server.Port = 119
	}
}

// knobFields maps admin-editable setting keys to the Config field each
// overrides. One place to keep the settings form, the save action, and the
// override resolution in sync.
func (c *Config) knobFields() map[string]*int {
	return map[string]*int{
		"retention_days":           &c.RetentionDays,
		"batch":                    &c.Batch,
		"max_groups":               &c.MaxGroups,
		"max_articles_per_group":   &c.MaxArticlesPerGroup,
		"backfill_batches_per_run": &c.BackfillBatchesPerRun,
	}
}

// withOverrides overlays DB settings (positive integers only) onto the config
// defaults. Invalid or missing values keep the default.
func (c Config) withOverrides(s map[string]string) Config {
	out := c
	for key, dst := range out.knobFields() {
		if raw, ok := s[key]; ok {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				*dst = n
			}
		}
	}
	return out
}
