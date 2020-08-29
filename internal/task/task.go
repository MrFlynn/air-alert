package task

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/urfave/cli/v2"
)

// Task interface declares the methods that a Task subtype should implement.
type Task interface {
	Run() error
	GetPriority() uint
	GetRate() interface{}
}

type priorityToTaskMap map[uint][]Task

// DailyTask is a Task that is run every day.
type DailyTask struct {
	TimeOfDay string
	Priority  uint
	RunFunc   func() error
}

func (d DailyTask) Run() error {
	return d.RunFunc()
}

func (d DailyTask) GetPriority() uint {
	return d.Priority
}

func (d DailyTask) GetRate() interface{} {
	return d.TimeOfDay
}

// MinuteTask is a Task that is run every n minutes where Rate defines
// how often the task is run.
type MinuteTask struct {
	Rate     uint64
	Priority uint
	RunFunc  func() error
}

func (m MinuteTask) Run() error {
	return m.RunFunc()
}

func (m MinuteTask) GetPriority() uint {
	return m.Priority
}

func (m MinuteTask) GetRate() interface{} {
	return m.Rate
}

// Runner is the main background task runner in this package.
type Runner struct {
	scheduler *gocron.Scheduler
	tasks     []Task
}

// NewRunner initializes a new Runner struct.
func NewRunner(ctx cli.Context) (Runner, error) {
	tz := ctx.String("timezone")

	location, err := time.LoadLocation(tz)
	if err != nil {
		return Runner{}, err
	}

	scheduler := gocron.NewScheduler(location)
	tasks := make([]Task, 0, 5)

	return Runner{
		scheduler: scheduler,
		tasks:     tasks,
	}, nil
}

// AddTask adds the task to the runner and schedules it.
func (r *Runner) AddTask(task Task) error {
	switch t := task.(type) {
	case DailyTask:
		r.scheduler.Every(1).Day().At(task.GetRate().(string)).Do(task.Run)
	case MinuteTask:
		r.scheduler.Every(task.GetRate().(uint64)).Minutes().Do(task.Run)
	default:
		return fmt.Errorf(`No scheduler for type %T`, t)
	}

	r.tasks = append(r.tasks, task)
	return nil
}

func (r *Runner) exposePriorities() priorityToTaskMap {
	taskMap := make(priorityToTaskMap, len(r.tasks))

	for _, task := range r.tasks {
		priority := task.GetPriority()
		taskMap[priority] = append(taskMap[priority], task)
	}

	return taskMap
}

func (r *Runner) runAllTasksInOrder() error {
	exposedPriorities := r.exposePriorities()

	orderedPriorities := make([]uint, len(exposedPriorities))
	for priority := range exposedPriorities {
		orderedPriorities = append(orderedPriorities, priority)
	}

	sort.Slice(orderedPriorities, func(i, j int) bool {
		return orderedPriorities[i] < orderedPriorities[j]
	})

	for _, priority := range orderedPriorities {
		for _, task := range exposedPriorities[priority] {
			if err := task.Run(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Start runs all tasks in descending order of priority (where 0 is the highest priority)
// or by insertion order if two tasks have the same priority. Then it starts a background
// thread which will run all tasks every day at the specified time.
func (r *Runner) Start() error {
	if err := r.runAllTasksInOrder(); err != nil {
		return fmt.Errorf(`Task failed during startup: %s`, err)
	}

	r.scheduler.StartAsync()
	return nil
}

// Stop stops the background thread.
func (r *Runner) Stop() {
	r.scheduler.Stop()
}