package main

import (
	"time"

	"github.com/spf13/viper"
)

// Build variables
var (
	version string
	commit  string
	date    string
)

func init() {
	buildDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		buildDate = time.Unix(0, 0)
	}

	viper.SetDefault("version", version)
	viper.SetDefault("commit", commit)
	viper.SetDefault("date", buildDate)
}
