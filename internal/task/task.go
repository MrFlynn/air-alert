package task

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/urfave/cli/v2"
)

type Task interface {
	Run() error
	GetPriority() uint
	GetTimeOfDay() string
}

type priorityToTaskMap map[uint][]Task

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

func (d DailyTask) GetTimeOfDay() string {
	return d.TimeOfDay
}

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

// Add task adds the task to the runner and schedules it.
func (r *Runner) AddTask(task Task) {
	r.tasks = append(r.tasks, task)
	r.scheduler.Every(1).Day().At(task.GetTimeOfDay()).Do(task.Run)
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