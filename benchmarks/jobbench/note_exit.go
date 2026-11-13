// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package jobbench

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// NoteExit appends a RunnerExit error event when the executor did not exit cleanly
// (Python jb_opencode_runner._note_exit).
func NoteExit(trajFile, logFile string, code int, limit string, kept bool) error {
	if code == 0 {
		return nil
	}
	span := limit
	if isDigits(limit) {
		span = limit + " s"
	}
	var why string
	switch {
	case code == 124:
		why = fmt.Sprintf("was stopped at the %s time limit (exit 124)", span)
	case code == -9 || code == 137:
		why = fmt.Sprintf("was killed with SIGKILL (%d), after the %s time limit or by the system", code, span)
	case code < 0:
		why = fmt.Sprintf("was killed by signal %d", -code)
	default:
		why = fmt.Sprintf("exited with status %d", code)
	}
	outcome := "the output directory was empty"
	if kept {
		outcome = "files already in the output directory were kept as the deliverable"
	}
	message := fmt.Sprintf("Rollout runner: the executor %s; %s.", why, outcome)
	event := map[string]any{
		"type":      "error",
		"timestamp": time.Now().UnixMilli(),
		"error": map[string]any{
			"name": "RunnerExit",
			"data": map[string]any{"message": message},
		},
	}
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(trajFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(append([]byte("\n"), append(b, '\n')...))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	if cerr != nil {
		return cerr
	}
	if logFile != "" {
		line := fmt.Sprintf("[%s] EXIT %d: %s\n", time.Now().Format("15:04:05"), code, message)
		lf, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = lf.WriteString(line)
			_ = lf.Close()
		}
	}
	return nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
