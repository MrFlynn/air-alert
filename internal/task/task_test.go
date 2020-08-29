package task

import (
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type FakeTask struct{}

func (f FakeTask) Run() error {
	return nil
}

func (f FakeTask) GetPriority() uint {
	return 0
}

func (f FakeTask) GetRate() interface{} {
	return nil
}

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
	secondTask = MinuteTask{
		Rate:     5,
		Priority: 2,
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

func TestAddTaskDaily(t *testing.T) {
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

	if !cmp.Equal(task, simpleRunner.tasks[0], cmpopts.IgnoreFields(DailyTask{}, "RunFunc")) {
		t.Errorf("Expected %+v\nGot %+v", task, simpleRunner.tasks[0])
	}

	if taskCount := simpleRunner.scheduler.Len(); taskCount != 1 {
		t.Errorf(`Expected 1 task, got %d tasks`, taskCount)
	}

	if err := simpleRunner.scheduler.Jobs()[0].Err(); err != nil {
		t.Errorf(`Got error during job creation: %s`, err)
	}
}

func TestAddTaskMinute(t *testing.T) {
	simpleRunner := Runner{
		scheduler: gocron.NewScheduler(tz),
		tasks:     make([]Task, 0, 5),
	}

	task := MinuteTask{
		Rate:     5,
		Priority: 1,
		RunFunc: func() error {
			return nil
		},
	}

	simpleRunner.AddTask(task)

	if count := len(simpleRunner.tasks); count != 1 {
		t.Errorf(`Expected 1 task, got %d tasks`, count)
	}

	if !cmp.Equal(task, simpleRunner.tasks[0], cmpopts.IgnoreFields(MinuteTask{}, "RunFunc")) {
		t.Errorf("Expected %+v\nGot %+v", task, simpleRunner.tasks[0])
	}

	if taskCount := simpleRunner.scheduler.Len(); taskCount != 1 {
		t.Errorf(`Expected 1 task, got %d tasks`, taskCount)
	}

	if err := simpleRunner.scheduler.Jobs()[0].Err(); err != nil {
		t.Errorf(`Got error during job creation: %s`, err)
	}
}

func TestAddFakeTask(t *testing.T) {
	simpleRunner := Runner{
		scheduler: gocron.NewScheduler(tz),
		tasks:     make([]Task, 0, 5),
	}

	task := FakeTask{}
	err := simpleRunner.AddTask(task)

	if err == nil {
		t.Errorf(`Expected error, got nil`)
	}

	if len(simpleRunner.tasks) != 0 {
		t.Error(`Task should not have been added if there was an error`)
	}
}

func TestExposePriorities(t *testing.T) {
	result := runner.exposePriorities()
	expected := map[uint][]Task{1: []Task{firstTask}, 2: []Task{secondTask}}

	for k, v := range result {
		if task, exists := expected[k]; exists {
			if !cmp.Equal(task, v, cmpopts.IgnoreFields(DailyTask{}, "RunFunc"), cmpopts.IgnoreFields(MinuteTask{}, "RunFunc")) {
				t.Errorf("Expected %+v\nGot%+v\nAt key %d", task, v, k)
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