package cuaserver

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

// gnuTools skips the test unless this machine has the GNU tools the Debian desktop has, such
// as stat -c, find -printf and dd iflag=skip_bytes. CI runs on Linux; macOS skips.
func gnuTools(t *testing.T, tools ...string) {
	t.Helper()
	checks := map[string][]string{
		"stat":    {"-c", "%s", "/"},
		"find":    {"/", "-maxdepth", "0", "-printf", "%f"},
		"dd":      {"if=/dev/null", "iflag=skip_bytes,count_bytes", "count=0", "status=none"},
		"timeout": {"1", "true"},
	}
	for _, tool := range tools {
		if err := exec.Command(tool, checks[tool]...).Run(); err != nil {
			t.Skipf("needs GNU %s: %v", tool, err)
		}
	}
}

// localDesk runs commands on this machine instead of in a desktop.
var localDesk = desk{name: "local", exec: func(ctx context.Context, stdin io.Reader, argv ...string) ([]byte, error) {
	return runner.Exec{}.Run(ctx, stdin, argv[0], argv[1:]...)
}}

func runLocal(t *testing.T, command string, p params) result {
	t.Helper()
	res, err := commandTable("test")[command].run(context.Background(), localDesk, p)
	if err != nil {
		t.Fatalf("%s %v: %v", command, p, err)
	}
	return res
}

func TestShellScriptRunsForReal(t *testing.T) {
	gnuTools(t, "stat", "timeout")
	res := runLocal(t, "run_command", params{"command": "printf out; echo err >&2; exit 3"})
	if !reflect.DeepEqual(res, result{"stdout": "out", "stderr": "err\n", "return_code": 3}) {
		t.Fatalf("res = %v", res)
	}
	// stderr survives a zero exit.
	if res := runLocal(t, "run_command", params{"command": "echo warn >&2"}); res["stderr"] != "warn\n" || res["return_code"] != 0 {
		t.Fatalf("res = %v", res)
	}
	// DISPLAY is set, as in Cua's in-session server.
	if res := runLocal(t, "run_command", params{"command": "echo $DISPLAY"}); res["stdout"] != ":1\n" {
		t.Fatalf("res = %v", res)
	}
	// A background process does not hold the command open.
	start := time.Now()
	runLocal(t, "run_command", params{"command": "sleep 5 &"})
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("a background process kept run_command waiting for %s", d)
	}
	// The timeout stops the command.
	start = time.Now()
	res = runLocal(t, "run_command", params{"command": "sleep 5", "timeout": 0.3})
	if res["success"] != false || res["return_code"] != -1 || time.Since(start) > 3*time.Second {
		t.Fatalf("res = %v after %s", res, time.Since(start))
	}
}

func TestFileScriptsRunForReal(t *testing.T) {
	gnuTools(t, "stat", "find", "dd")
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	runLocal(t, "write_text", params{"path": f, "content": "hello"})
	runLocal(t, "write_bytes", params{"path": f, "content_b64": b64([]byte(" world")), "append": true})
	if res := runLocal(t, "read_text", params{"path": f}); res["content"] != "hello world" {
		t.Fatalf("read_text = %v", res)
	}
	if res := runLocal(t, "read_bytes", params{"path": f, "offset": 6.0, "length": 3.0}); res["content_b64"] != b64([]byte("wor")) {
		t.Fatalf("read_bytes range = %v", res)
	}
	if res := runLocal(t, "read_bytes", params{"path": f, "offset": 6.0}); res["content_b64"] != b64([]byte("world")) {
		t.Fatalf("read_bytes from offset = %v", res)
	}
	if res := runLocal(t, "get_file_size", params{"path": f}); res["size"] != int64(11) {
		t.Fatalf("size = %v", res)
	}
	sub := filepath.Join(dir, "sub", "deeper")
	runLocal(t, "create_dir", params{"path": sub})
	checks := []struct {
		command, path string
		want          bool
	}{
		{"file_exists", f, true}, {"file_exists", dir, false}, {"directory_exists", sub, true}, {"directory_exists", f, false},
	}
	for _, c := range checks {
		if res := runLocal(t, c.command, params{"path": c.path}); res["exists"] != c.want {
			t.Errorf("%s %s = %v", c.command, c.path, res)
		}
	}
	if res := runLocal(t, "list_dir", params{"path": dir}); !reflect.DeepEqual(res["files"], []string{"a.txt", "sub"}) {
		t.Fatalf("list_dir = %v", res)
	}
	// A symlinked directory is listed through the link, as /bin is on Debian.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if res := runLocal(t, "list_dir", params{"path": link}); !reflect.DeepEqual(res["files"], []string{"a.txt", "sub"}) {
		t.Fatalf("list_dir through a symlink = %v", res)
	}
	runLocal(t, "delete_file", params{"path": f})
	runLocal(t, "delete_dir", params{"path": sub})
	if _, err := commandTable("test")["delete_dir"].run(context.Background(), localDesk, params{"path": dir}); err == nil {
		t.Fatal("delete_dir removed a directory that is not empty")
	}
	if _, err := commandTable("test")["read_text"].run(context.Background(), localDesk, params{"path": f}); err == nil {
		t.Fatal("read_text of a deleted file succeeded")
	}
}
