package smolvm

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

// Error is a failure reported by smolvm itself (not by a command running inside a machine).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return "smolvm: " + e.Code + ": " + e.Message }

// IsNotFound reports whether err is smolvm's NOT_FOUND error.
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == "NOT_FOUND"
}

// parseError turns smolvm's JSON error line into *Error. Anything else is returned unchanged,
// which keeps a guest command's own exit code and stderr visible to callers.
func parseError(err error) error {
	var ee *runner.ExitError
	if !errors.As(err, &ee) {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(ee.Stderr)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	var payload struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(last), &payload) == nil && payload.Error != nil && payload.Error.Code != "" {
		return &Error{Code: payload.Error.Code, Message: payload.Error.Message}
	}
	return err
}
