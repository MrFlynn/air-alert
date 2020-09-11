package task

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Task interface declares the methods that a Task subtype should implement.
type Task interface {
	Run(context.Context) error
	GetPriority() uint
	GetRate() interface{}
	GetTTL() time.Duration
	SkipStartup() bool
}

// WrapTimeout wraps the `Run` interface method with a timeout-dependent context.
func WrapTimeout(t Task, noLog bool) error {
	var err error

	// Create context without timeout equal to given time-to-live.
	ctx, cancel := context.WithTimeout(context.Background(), t.GetTTL())
	defer cancel()

	taskTermination := make(chan bool, 1)

	go func() {
		err = t.Run(ctx)
		taskTermination <- true
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-taskTermination:
		if err != nil {
			if !noLog {
				log.Error(err)
			}

			return err
		}
	}

	return nil
}

type priorityToTaskMap map[uint][]Task

// DailyTask is a Task that is run every day.
type DailyTask struct {
	TimeOfDay string
	Priority  uint
	TTL       time.Duration
	SkipStart bool
	RunFunc   func(context.Context) error
}

// Run wraps the internal RunFunc field. This method runs it and returns the result.
func (d DailyTask) Run(ctx context.Context) error {
	return d.RunFunc(ctx)
}

// GetPriority returns the task priority.
func (d DailyTask) GetPriority() uint {
	return d.Priority
}

// GetRate returns the repeat frequency for the task.
func (d DailyTask) GetRate() interface{} {
	return d.TimeOfDay
}

// GetTTL returns the maximum duration of the task. This parameter is to prevent the task
// from running too long if it hangs.
func (d DailyTask) GetTTL() time.Duration {
	return d.TTL
}

// SkipStartup returns whether or not the task should be skipped during the initial startup phase.
func (d DailyTask) SkipStartup() bool {
	return d.SkipStart
}

// MinuteTask is a Task that is run every n minutes where Rate defines
// how often the task is run.
type MinuteTask struct {
	Rate      uint64
	Priority  uint
	TTL       time.Duration
	SkipStart bool
	RunFunc   func(context.Context) error
}

// Run wraps the internal RunFunc field. This method runs it and returns the result.
func (m MinuteTask) Run(ctx context.Context) error {
	return m.RunFunc(ctx)
}

// GetPriority returns the task priority.
func (m MinuteTask) GetPriority() uint {
	return m.Priority
}

// GetRate returns the repeat frequency for the task.
func (m MinuteTask) GetRate() interface{} {
	return m.Rate
}

// GetTTL returns the maximum duration of the task. This parameter is to prevent the task
// from running too long if it hangs.
func (m MinuteTask) GetTTL() time.Duration {
	return m.TTL
}

// SkipStartup returns whether or not the task should be skipped during the initial startup phase.
func (m MinuteTask) SkipStartup() bool {
	return m.SkipStart
}

// Runner is the main background task runner in this package.
type Runner struct {
	scheduler *gocron.Scheduler
	tasks     []Task
}

// NewRunner initializes a new Runner struct.
func NewRunner() (*Runner, error) {
	location, err := time.LoadLocation(viper.GetString("timezone"))
	if err != nil {
		return &Runner{}, err
	}

	scheduler := gocron.NewScheduler(location)
	tasks := make([]Task, 0, 5)

	return &Runner{
		scheduler: scheduler,
		tasks:     tasks,
	}, nil
}

// AddTask adds the task to the runner and schedules it.
func (r *Runner) AddTask(task Task) error {
	switch t := task.(type) {
	case DailyTask:
		r.scheduler.Every(1).Day().At(task.GetRate().(string)).Do(WrapTimeout, task, false)
	case MinuteTask:
		r.scheduler.Every(task.GetRate().(uint64)).Minutes().Do(WrapTimeout, task, false)
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
			if !task.SkipStartup() {
				if err := WrapTimeout(task, true); err != nil {
					return err
				}
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
		return fmt.Errorf(`task failed during startup: %s`, err)
	}

	r.scheduler.StartAsync()
	return nil
}

// Stop stops the background thread.
func (r *Runner) Stop() {
	r.scheduler.Stop()
}
