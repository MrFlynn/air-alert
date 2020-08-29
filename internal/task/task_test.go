package task

import (
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	tz = time.UTC

	firstChan  = make(chan int64, 1)
	secondChan = make(chan int64, 1)

	firstTask = DailyTask{
		TimeOfDay: "10:30",
		Priority:  1,
		RunFunc: func() error {
			firstChan <- time.Now().UnixNano()
			time.Sleep(10 * time.Millisecond) // Prevent code from running too fast.
			return nil
		},
	}
	secondTask = DailyTask{
		TimeOfDay: "20:30",
		Priority:  2,
		RunFunc: func() error {
			secondChan <- time.Now().UnixNano()
			return nil
		},
	}
	runner = Runner{
		scheduler: gocron.NewScheduler(tz),
		tasks:     []Task{firstTask, secondTask},
	}
)

func TestAddTask(t *testing.T) {
	simpleRunner := Runner{
		scheduler: gocron.NewScheduler(tz),
		tasks:     make([]Task, 0, 5),
	}

	task := DailyTask{
		TimeOfDay: "10:30",
		Priority:  1,
		RunFunc: func() error {
			return nil
		},
	}

	simpleRunner.AddTask(task)

	if count := len(simpleRunner.tasks); count != 1 {
		t.Errorf(`Expected 1 task, got %d tasks`, count)
	}

	if taskCount := simpleRunner.scheduler.Len(); taskCount != 1 {
		t.Errorf(`Expected 1 task, got %d tasks`, taskCount)
	}
}

func TestExposePriorities(t *testing.T) {
	result := runner.exposePriorities()
	expected := map[uint][]Task{1: []Task{firstTask}, 2: []Task{secondTask}}

	for k, v := range result {
		if task, exists := expected[k]; exists {
			if !cmp.Equal(task, v, cmpopts.IgnoreFields(DailyTask{}, "RunFunc")) {
				t.Errorf("Expected: %+v\nGot:%+v\nAt key %d", task, v, k)
			}
		} else {
			t.Errorf(`Key %d in expected not found, but should have been found`, k)
		}
	}
}

func TestRunAllTasksInOrder(t *testing.T) {
	err := runner.runAllTasksInOrder()

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	firstTime := <-firstChan
	secondTime := <-secondChan

	if firstTime >= secondTime {
		t.Error(`Tasks run out of order. Expected firstTask to run first`)
	}
}