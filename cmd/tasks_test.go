// +build unit

package cmd

import (
	"testing"
	"time"

	"github.com/mrflynn/air-alert/internal/database/redis"
)

func TestFindCrossover(t *testing.T) {
	data := []*redis.RawQualityData{
		{
			Time: 20,
			AQI:  60.0,
		},
		{
			Time: 15,
			AQI:  55.0,
		},
		{
			Time: 10,
			AQI:  53.0,
		},
		{
			Time: 5,
			AQI:  43.0,
		},
		{
			Time: 0,
			AQI:  40,
		},
	}

	_, crossoverExact := findCrossover(data, 55.0)

	if !time.Unix(15, 0).Equal(crossoverExact) {
		t.Errorf("expected time to be 15, got %d", crossoverExact.Unix())
	}

	_, crossoverBetween := findCrossover(data, 52.0)

	if !time.Unix(5, 0).Equal(crossoverBetween) {
		t.Errorf("expected time to be 5, got %d", crossoverBetween.Unix())
	}

	_, crossoverBelow := findCrossover(data, 30)

	if !(time.Time{}).Equal(crossoverBelow) {
		t.Errorf("expected time to be 0, got %d", crossoverBelow.Unix())
	}

	_, crossoverAbove := findCrossover(data, 100)

	if !(time.Time{}).Equal(crossoverAbove) {
		t.Errorf("expected time to be 0, got %d", crossoverAbove.Unix())
	}
}
