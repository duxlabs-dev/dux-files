package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
)

const cliScope = "cli"

type jobEvent struct {
	Kind  string        `json:"kind"`
	JobID string        `json:"jobId"`
	Job   *fileops.View `json:"job"`
}

type jobResult struct {
	Job    fileops.View `json:"job"`
	Failed []string     `json:"failed"`
}

func runJobUnattended(s *session, method string, params map[string]any, onConflict string) (jobResult, error) {
	if err := s.subscribe("jobs"); err != nil {
		return jobResult{}, err
	}
	params["scope"] = cliScope
	var started struct {
		Job fileops.View `json:"job"`
	}
	if err := s.call(method, params, &started); err != nil {
		return jobResult{}, err
	}

	result := jobResult{Job: started.Job, Failed: []string{}}
	answered := ""
	for ev := range s.events {
		if ev.Topic != "jobs" {
			continue
		}
		var payload jobEvent
		if json.Unmarshal(ev.Data, &payload) != nil || payload.Kind != "job" || payload.JobID != started.Job.ID || payload.Job == nil {
			continue
		}
		job := *payload.Job
		result.Job = job
		switch job.State {
		case fileops.StateDone, fileops.StateFailed:
			return result, nil
		case fileops.StatePausedConflict:
			if job.Conflict == nil || job.Conflict.ID == answered {
				continue
			}
			answered = job.Conflict.ID
			s.answer(job.ID, job.Conflict.ID, pick(job.Conflict.Actions, onConflict))
		case fileops.StatePausedError:
			if job.Failure == nil || job.Failure.ID == answered {
				continue
			}
			answered = job.Failure.ID
			fmt.Fprintf(os.Stderr, "dfiles: %s: %s\n", job.Failure.Path, job.Failure.Message)
			result.Failed = append(result.Failed, job.Failure.Path)
			s.answer(job.ID, job.Failure.ID, pick(job.Failure.Actions, "skip"))
		case fileops.StatePausedPassword:
			if job.PasswordPrompt == nil || job.PasswordPrompt.ID == answered {
				continue
			}
			answered = job.PasswordPrompt.ID
			fmt.Fprintf(os.Stderr, "dfiles: %s: needs a password\n", job.PasswordPrompt.Path)
			result.Failed = append(result.Failed, job.PasswordPrompt.Path)
			s.answer(job.ID, job.PasswordPrompt.ID, pick(job.PasswordPrompt.Actions, "skip"))
		}
	}
	return result, errors.New("lost the connection while the job ran")
}

func pick(actions []fileops.Action, preferred string) string {
	if slices.Contains(actions, fileops.Action(preferred)) {
		return preferred
	}
	if slices.Contains(actions, fileops.Action("skip")) {
		return "skip"
	}
	return "cancel"
}

func (s *session) answer(jobID, id, action string) {
	_ = s.call("files.op.respond", map[string]any{"jobId": jobID, "id": id, "action": action}, nil)
}

func (r jobResult) err() error {
	switch {
	case r.Job.State == fileops.StateFailed:
		return fmt.Errorf("%s failed: %s", r.Job.Kind, r.Job.Error)
	case r.Job.Cancelled:
		return fmt.Errorf("%s was cancelled", r.Job.Kind)
	case len(r.Failed) > 0:
		return fmt.Errorf("%s failed for %d item(s)", r.Job.Kind, len(r.Failed))
	}
	return nil
}

func (r jobResult) succeeded(source string) bool {
	for _, failed := range r.Failed {
		if pathutil.Within(failed, source) {
			return false
		}
	}
	return true
}

var conflictActions = map[string]string{"keep-both": "keepBoth", "skip": "skip", "replace": "replace"}

func conflictAction(flag string) (string, error) {
	action, ok := conflictActions[flag]
	if !ok {
		return "", fmt.Errorf("--conflict must be keep-both, skip or replace, not %q", flag)
	}
	return action, nil
}
