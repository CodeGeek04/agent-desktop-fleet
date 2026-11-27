package main

import "github.com/CodeGeek04/agent-desktop-fleet/internal/activity"

// activityLog returns the shared activity log, or nil when no config directory exists.
func activityLog() *activity.Log {
	p, err := activity.DefaultPath()
	if err != nil {
		return nil
	}
	return &activity.Log{Path: p}
}
