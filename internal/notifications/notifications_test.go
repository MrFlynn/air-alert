// +build unit

package notifications

import (
	"reflect"
	"testing"

	"github.com/mrflynn/air-alert/internal/database/redis"
)

func TestCreateNotificationText(t *testing.T) {
	notification := redis.NotificationStream{
		AQI:      50.125,
		Forecast: redis.AQIIncreasing,
	}

	expected := []byte("The AQI is 50.1. Time to go inside.")
	result := createNotificationText(notification)

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("expected notification text to be %s, got %s", expected, result)
	}

	notification = redis.NotificationStream{
		AQI:      62.367,
		Forecast: redis.AQIDecreasing,
	}

	expected = []byte("The AQI is 62.4. Time to get some fresh air!")
	result = createNotificationText(notification)

	if !reflect.DeepEqual(expected, result) {
		t.Errorf("expected notification text to be %s, got %s", expected, result)
	}
}
