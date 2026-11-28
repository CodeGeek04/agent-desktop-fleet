package cuaserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

// shellScript runs "$1" with sh and prints "CODE STDOUT_BYTES STDERR_BYTES" on one line,
// then exactly that much stdout and stderr. Collecting both in files keeps stderr when the
// command succeeds, which smolvm exec would otherwise drop, and keeps the exec from waiting
// on a background process that inherited its output. "$2", when set, is a timeout in
// seconds, enforced inside the desktop so the command really stops.
const shellScript = `o=$(mktemp) && e=$(mktemp) || exit 125
if [ -n "$2" ]; then timeout -k 5 "$2" sh -c "$1"; else sh -c "$1"; fi >"$o" 2>"$e" </dev/null
rc=$?
no=$(stat -c %s "$o") ne=$(stat -c %s "$e")
printf '%s %s %s\n' "$rc" "$no" "$ne"
head -c "$no" "$o"
head -c "$ne" "$e"
rm -f "$o" "$e"`

func shellCommands() map[string]command {
	return map[string]command{
		"run_command": {params: []param{req("command"), opt("timeout", nil)}, need: needAwake, run: runCommand,
			detail: func(p params) string { c, _ := p.strOr("command", ""); return clip(c) }},
	}
}

// runCommand runs a shell command as root inside the desktop, with DISPLAY set like Cua's
// in-session server had it. A non-zero exit is still a success, as in Cua.
func runCommand(ctx context.Context, d desk, p params) (result, error) {
	command, err := p.mustStr("command")
	if err != nil {
		return nil, err
	}
	secs, hasTimeout, err := p.number("timeout")
	if err != nil {
		return nil, err
	}
	limit := ""
	if hasTimeout {
		if secs <= 0 {
			return nil, fmt.Errorf("timeout must be more than 0 seconds, got %g", secs)
		}
		limit = strconv.FormatFloat(secs, 'f', -1, 64)
		// A backstop in case the desktop stops answering; the timeout inside fires first.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(secs*float64(time.Second))+15*time.Second)
		defer cancel()
	}
	start := time.Now()
	out, err := d.exec(ctx, nil, "env", "DISPLAY="+image.Display, "sh", "-c", shellScript, "sh", command, limit)
	if err != nil {
		if hasTimeout && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return timedOut(secs), nil
		}
		return nil, guestErr(err)
	}
	code, stdout, stderr, err := parseShell(out)
	if err != nil {
		return nil, err
	}
	// timeout(1) exits 124, or 137 when the command had to be killed.
	if hasTimeout && (code == 124 || code == 137) && time.Since(start).Seconds() >= secs {
		return timedOut(secs), nil
	}
	return result{"stdout": stdout, "stderr": stderr, "return_code": code}, nil
}

// timedOut is Cua's answer for a command that ran out of time, plus an error that
// cua-computer can show (it reports only "error" when success is false).
func timedOut(secs float64) result {
	msg := fmt.Sprintf("Command timed out after %gs", secs)
	return result{"success": false, "stdout": "", "stderr": msg, "return_code": -1, "error": msg}
}

func parseShell(out []byte) (code int, stdout, stderr string, err error) {
	head, body, ok := bytes.Cut(out, []byte("\n"))
	f := strings.Fields(string(head))
	bad := fmt.Errorf("unexpected output from the desktop's shell: %q", clip(string(out)))
	if !ok || len(f) != 3 {
		return 0, "", "", bad
	}
	code, err1 := strconv.Atoi(f[0])
	no, err2 := strconv.Atoi(f[1])
	ne, err3 := strconv.Atoi(f[2])
	if err1 != nil || err2 != nil || err3 != nil || no < 0 || ne < 0 || no+ne != len(body) {
		return 0, "", "", bad
	}
	return code, string(body[:no]), string(body[no:]), nil
}

// guestErr turns a failed command inside the desktop into its own error message.
func guestErr(err error) error {
	var ee *runner.ExitError
	if errors.As(err, &ee) {
		if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
			return errors.New(msg)
		}
		return fmt.Errorf("exited with code %d", ee.Code)
	}
	return err
}
