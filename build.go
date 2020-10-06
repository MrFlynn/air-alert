package main

import (
	"time"

	"github.com/mrflynn/air-alert/cmd"
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

	cmd.ProgramInfoStore.SetDefault("version", version)
	cmd.ProgramInfoStore.SetDefault("commit", commit)
	cmd.ProgramInfoStore.SetDefault("date", buildDate)
}
