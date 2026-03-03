package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TaskState represents the persistent state of a scheduled task.
type TaskState struct {
	Name        string     `json:"name"`
	Schedule    string     `json:"schedule"`    // interval (e.g., "24h") or time (e.g., "03:00")
	LastRun     *time.Time `json:"lastRun"`
	NextRun     *time.Time `json:"nextRun"`
	IsRunning   bool       `json:"isRunning"`
	LastError   string     `json:"lastError"`
	RunCount    int        `json:"runCount"`
}

// HistoryEntry is a single run log entry.
type HistoryEntry struct {
	TaskName  string    `json:"taskName"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Duration  string    `json:"duration"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// Scheduler manages persistent task scheduling.
type Scheduler struct {
	mu          sync.RWMutex
	tasks       map[string]*scheduledTask
	stateDir    string
	logsDir     string
	savedStates map[string]TaskState // from previous run, for re-registration
}

type scheduledTask struct {
	state   TaskState
	handler func() error
	timer   *time.Timer
	ticker  *time.Ticker
	stop    chan struct{}
}

// New creates a scheduler with persistent state.
func New() *Scheduler {
	stateDir := "/app/data/scheduler"
	if _, err := os.Stat("/app/data"); os.IsNotExist(err) {
		stateDir = ".runtime/scheduler"
	}

	s := &Scheduler{
		tasks:    make(map[string]*scheduledTask),
		stateDir: stateDir,
		logsDir:  filepath.Join(stateDir, "logs"),
	}
	os.MkdirAll(s.logsDir, 0o755)
	s.loadState()
	return s
}

// Schedule schedules a task with the given schedule string.
// Schedule can be an interval ("24h", "6h") or a daily time ("03:00" in UTC).
func (s *Scheduler) Schedule(name, schedule string, handler func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing task if any.
	if existing, ok := s.tasks[name]; ok {
		close(existing.stop)
		if existing.ticker != nil {
			existing.ticker.Stop()
		}
		if existing.timer != nil {
			existing.timer.Stop()
		}
	}

	now := time.Now().UTC()
	state := TaskState{
		Name:     name,
		Schedule: schedule,
	}

	task := &scheduledTask{
		state:   state,
		handler: handler,
		stop:    make(chan struct{}),
	}

	// Parse schedule.
	if d, err := time.ParseDuration(schedule); err == nil {
		// Interval-based.
		next := now.Add(d)
		task.state.NextRun = &next
		task.ticker = time.NewTicker(d)
		go s.runInterval(name, task)
	} else if len(schedule) == 5 && schedule[2] == ':' {
		// Time-based (HH:MM UTC).
		next := s.nextDailyTime(schedule)
		task.state.NextRun = &next
		go s.runDaily(name, task, schedule)
	} else {
		return fmt.Errorf("invalid schedule: %s (use duration like '24h' or time like '03:00')", schedule)
	}

	s.tasks[name] = task
	s.saveState()

	log.Printf("[scheduler] Scheduled %q: %s (next: %v)", name, schedule, task.state.NextRun)
	return nil
}

// Unschedule removes a scheduled task.
func (s *Scheduler) Unschedule(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.tasks[name]; ok {
		close(task.stop)
		if task.ticker != nil {
			task.ticker.Stop()
		}
		if task.timer != nil {
			task.timer.Stop()
		}
		delete(s.tasks, name)
		s.saveState()
		log.Printf("[scheduler] Unscheduled %q", name)
	}
}

// GetTask returns the state of a task.
func (s *Scheduler) GetTask(name string) *TaskState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if task, ok := s.tasks[name]; ok {
		state := task.state
		return &state
	}
	return nil
}

// GetAllTasks returns all task states.
func (s *Scheduler) GetAllTasks() []TaskState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var states []TaskState
	for _, task := range s.tasks {
		states = append(states, task.state)
	}
	return states
}

// ReadHistory reads the last N history entries for a task.
func (s *Scheduler) ReadHistory(name string, limit int) []HistoryEntry {
	historyFile := filepath.Join(s.logsDir, name+".jsonl")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil
	}

	var entries []HistoryEntry
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	// Return last N entries.
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries
}

func (s *Scheduler) runInterval(name string, task *scheduledTask) {
	for {
		select {
		case <-task.stop:
			return
		case <-task.ticker.C:
			s.executeTask(name, task)
		}
	}
}

func (s *Scheduler) runDaily(name string, task *scheduledTask, timeStr string) {
	for {
		next := s.nextDailyTime(timeStr)
		delay := time.Until(next)
		if delay < 0 {
			delay = 24 * time.Hour
		}

		timer := time.NewTimer(delay)
		select {
		case <-task.stop:
			timer.Stop()
			return
		case <-timer.C:
			s.executeTask(name, task)
		}
	}
}

func (s *Scheduler) executeTask(name string, task *scheduledTask) {
	s.mu.Lock()
	task.state.IsRunning = true
	s.mu.Unlock()

	startTime := time.Now()
	err := task.handler()
	endTime := time.Now()

	s.mu.Lock()
	task.state.IsRunning = false
	now := endTime
	task.state.LastRun = &now
	task.state.RunCount++
	if err != nil {
		task.state.LastError = err.Error()
	} else {
		task.state.LastError = ""
	}
	s.saveState()
	s.mu.Unlock()

	// Log history.
	entry := HistoryEntry{
		TaskName:  name,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime).String(),
		Success:   err == nil,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	s.appendHistory(name, entry)

	if err != nil {
		log.Printf("[scheduler] Task %q failed: %v", name, err)
	} else {
		log.Printf("[scheduler] Task %q completed in %v", name, endTime.Sub(startTime))
	}
}

func (s *Scheduler) nextDailyTime(timeStr string) time.Time {
	now := time.Now().UTC()
	var hour, minute int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)

	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// SavedStates returns the persisted task states from the previous run.
// Use this to re-register handlers after a restart.
func (s *Scheduler) SavedStates() map[string]TaskState {
	return s.savedStates
}

func (s *Scheduler) loadState() {
	stateFile := filepath.Join(s.stateDir, "scheduler-state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}

	var states map[string]TaskState
	if err := json.Unmarshal(data, &states); err != nil {
		log.Printf("[scheduler] Failed to load state: %v", err)
		return
	}
	s.savedStates = states
}

func (s *Scheduler) saveState() {
	os.MkdirAll(s.stateDir, 0o755)
	stateFile := filepath.Join(s.stateDir, "scheduler-state.json")

	states := make(map[string]TaskState)
	for name, task := range s.tasks {
		states[name] = task.state
	}

	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(stateFile, data, 0o644)
}

func (s *Scheduler) appendHistory(name string, entry HistoryEntry) {
	historyFile := filepath.Join(s.logsDir, name+".jsonl")
	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(entry)
	f.Write(data)
	f.WriteString("\n")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
