// +build unit

package notifications

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrflynn/air-alert/internal/database/redis"
)

var countStopped uint64

func (s *Sender) testDispatch() {
	<-s.stop
	s.ack <- true
	atomic.AddUint64(&countStopped, 1)
}

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

func TestSenderShutdown(t *testing.T) {
	sender := NewSender(nil, nil)
	numThreads := 5
	sender.Threads = uint(numThreads)

	for i := 0; i < numThreads; i++ {
		go sender.testDispatch()
	}

	sender.Shutdown()

	// Need time for counts to propogate.
	time.Sleep(time.Millisecond)

	if int(countStopped) != numThreads {
		t.Errorf("expected number of canceled threads to be %d, actually canceled %d", numThreads, countStopped)
	}
}
