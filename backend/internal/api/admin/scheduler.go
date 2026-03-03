package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"anigraph/backend/internal/api/httputil"
	"anigraph/backend/internal/scheduler"
)

var globalScheduler *scheduler.Scheduler

func getScheduler() *scheduler.Scheduler {
	if globalScheduler == nil {
		globalScheduler = scheduler.New()
	}
	return globalScheduler
}

// RestoreSchedules re-registers any schedules that were active before a restart.
func (h *Handler) RestoreSchedules() {
	sched := getScheduler()
	for name, state := range sched.SavedStates() {
		if state.Schedule == "" {
			continue
		}
		log.Printf("[scheduler] Restoring schedule %q: %s", name, state.Schedule)
		mode := "full" // default mode
		err := sched.Schedule(name, state.Schedule, func() error {
			log.Printf("[scheduler] Running restored scheduled task %q (mode: %s)", name, mode)
			results := h.runPipeline(mode, false, 16)
			if summary, ok := results["summary"].(map[string]any); ok {
				if status, ok := summary["overallStatus"].(string); ok && status == "failed" {
					return fmt.Errorf("pipeline failed")
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("[scheduler] Failed to restore %q: %v", name, err)
		}
	}
}

// ScheduleIncrementalUpdate manages scheduled pipeline execution.
func (h *Handler) ScheduleIncrementalUpdate(w http.ResponseWriter, r *http.Request) {
	sched := getScheduler()

	var body struct {
		Action   string `json:"action"`   // schedule, unschedule, status
		Schedule string `json:"schedule"` // e.g., "24h", "03:00"
		Mode     string `json:"mode"`     // full, scraper-only, data-only
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Default to status check.
		body.Action = "status"
	}

	taskName := "incremental-update"

	switch body.Action {
	case "schedule":
		if body.Schedule == "" {
			httputil.Error(w, http.StatusBadRequest, "schedule is required (e.g., '24h', '03:00')")
			return
		}

		mode := body.Mode
		if mode == "" {
			mode = "full"
		}

		err := sched.Schedule(taskName, body.Schedule, func() error {
			log.Printf("[scheduler] Running scheduled incremental update (mode: %s)", mode)
			results := h.runPipeline(mode, false, 16)
			if summary, ok := results["summary"].(map[string]any); ok {
				if status, ok := summary["overallStatus"].(string); ok && status == "failed" {
					return fmt.Errorf("pipeline failed")
				}
			}
			return nil
		})

		if err != nil {
			httputil.Error(w, http.StatusBadRequest, err.Error())
			return
		}

		httputil.JSON(w, http.StatusOK, map[string]any{
			"success":  true,
			"message":  fmt.Sprintf("Scheduled %s with schedule %q", taskName, body.Schedule),
			"schedule": body.Schedule,
			"mode":     mode,
		})

	case "unschedule":
		sched.Unschedule(taskName)
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Unscheduled incremental update",
		})

	default: // "status" or empty
		task := sched.GetTask(taskName)
		if task == nil {
			httputil.JSON(w, http.StatusOK, map[string]any{
				"success":   true,
				"scheduled": false,
				"message":   "No scheduled incremental update",
			})
			return
		}

		httputil.JSON(w, http.StatusOK, map[string]any{
			"success":   true,
			"scheduled": true,
			"task":      task,
		})
	}
}

// IncrementalUpdateLogs retrieves task run history.
func (h *Handler) IncrementalUpdateLogs(w http.ResponseWriter, r *http.Request) {
	sched := getScheduler()
	limit := httputil.QueryInt(r, "limit", 50)
	includeHistory := r.URL.Query().Get("history") == "true"

	taskName := "incremental-update"
	task := sched.GetTask(taskName)

	response := map[string]any{
		"success": true,
		"task":    task,
	}

	if includeHistory {
		history := sched.ReadHistory(taskName, limit)
		response["history"] = history
	}

	httputil.JSON(w, http.StatusOK, response)
}
