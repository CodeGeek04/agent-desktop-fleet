package cuaserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Each file command is a small script run as `sh -c SCRIPT sh PATH [ARG]`, with file data
// on stdin or stdout, so the desktop image needs nothing new. They run as root, like
// run_command, and behave like Cua's Python handlers.
const (
	fileExistsScript = `if [ -f "$1" ]; then echo yes; else echo no; fi`
	dirExistsScript  = `if [ -d "$1" ]; then echo yes; else echo no; fi`
	// Cua listed entries that are files or directories, following symlinks. -H follows "$1"
	// itself when it is a symlink, as /bin is on Debian.
	listDirScript = `[ -d "$1" ] || { echo "not a directory: $1" >&2; exit 1; }
find -H "$1" -mindepth 1 -maxdepth 1 \( -xtype f -o -xtype d \) -printf '%f\0'`
	readScript = `exec cat -- "$1"`
	// "$2" is the byte offset and "$3", when set, the number of bytes.
	readRangeScript = `exec dd if="$1" bs=65536 iflag=skip_bytes,count_bytes skip="$2" ${3:+count="$3"} status=none`
	writeScript     = `cat > "$1"`
	appendScript    = `cat >> "$1"`
	sizeScript      = `exec stat -c %s -- "$1"`
	deleteScript    = `exec rm -- "$1"`
	mkdirScript     = `exec mkdir -p -- "$1"`
	rmdirScript     = `exec rmdir -- "$1"`
)

// homeDir is where "~" points: commands in the desktop run as root.
const homeDir = "/root"

func fileCommands() map[string]command {
	path := []param{req("path")}
	cmd := func(ps []param, run func(context.Context, desk, string, params) (result, error)) command {
		return command{params: ps, need: needAwake, detail: pathDetail,
			run: func(ctx context.Context, d desk, p params) (result, error) {
				file, err := filePath(p)
				if err != nil {
					return nil, err
				}
				return run(ctx, d, file, p)
			}}
	}
	yesNo := func(script string) command {
		return cmd(path, func(ctx context.Context, d desk, file string, _ params) (result, error) {
			out, err := sh(ctx, d, nil, script, file)
			return result{"exists": strings.TrimSpace(string(out)) == "yes"}, err
		})
	}
	simple := func(script string) command {
		return cmd(path, func(ctx context.Context, d desk, file string, _ params) (result, error) {
			_, err := sh(ctx, d, nil, script, file)
			return nil, err
		})
	}
	return map[string]command{
		"file_exists":      yesNo(fileExistsScript),
		"directory_exists": yesNo(dirExistsScript),
		"list_dir": cmd(path, func(ctx context.Context, d desk, dir string, _ params) (result, error) {
			out, err := sh(ctx, d, nil, listDirScript, dir)
			if err != nil {
				return nil, err
			}
			names := []string{}
			for _, n := range bytes.Split(out, []byte{0}) {
				if len(n) > 0 {
					names = append(names, string(n))
				}
			}
			sort.Strings(names)
			return result{"files": names}, nil
		}),
		"read_text": cmd(path, func(ctx context.Context, d desk, file string, _ params) (result, error) {
			out, err := sh(ctx, d, nil, readScript, file)
			return result{"content": string(out)}, err
		}),
		"write_text": cmd([]param{req("path"), req("content")}, func(ctx context.Context, d desk, file string, p params) (result, error) {
			content, err := p.mustStr("content")
			if err != nil {
				return nil, err
			}
			_, err = sh(ctx, d, []byte(content), writeScript, file)
			return nil, err
		}),
		"read_bytes": cmd([]param{req("path"), opt("offset", 0), opt("length", nil)}, readBytes),
		"write_bytes": cmd([]param{req("path"), req("content_b64"), opt("append", false)}, func(ctx context.Context, d desk, file string, p params) (result, error) {
			encoded, err := p.mustStr("content_b64")
			if err != nil {
				return nil, err
			}
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("content_b64 is not valid base64: %w", err)
			}
			appendTo, err := p.boolOr("append", false)
			if err != nil {
				return nil, err
			}
			_, err = sh(ctx, d, data, pick(appendTo, appendScript, writeScript), file)
			return nil, err
		}),
		"get_file_size": cmd(path, func(ctx context.Context, d desk, file string, _ params) (result, error) {
			out, err := sh(ctx, d, nil, sizeScript, file)
			if err != nil {
				return nil, err
			}
			n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("unexpected size %q for %s", clip(string(out)), file)
			}
			return result{"size": n}, nil
		}),
		"delete_file": simple(deleteScript),
		"create_dir":  simple(mkdirScript),
		"delete_dir":  simple(rmdirScript),
	}
}

func readBytes(ctx context.Context, d desk, file string, p params) (result, error) {
	offset, err := p.intOr("offset", 0)
	if err != nil {
		return nil, err
	}
	length, hasLength, err := p.int("length")
	if err != nil {
		return nil, err
	}
	if offset < 0 || length < 0 {
		return nil, fmt.Errorf("offset and length must not be negative")
	}
	var out []byte
	if offset == 0 && !hasLength {
		out, err = sh(ctx, d, nil, readScript, file)
	} else {
		count := ""
		if hasLength {
			count = strconv.Itoa(length)
		}
		out, err = sh(ctx, d, nil, readRangeScript, file, strconv.Itoa(offset), count)
	}
	if err != nil {
		return nil, err
	}
	return result{"content_b64": base64.StdEncoding.EncodeToString(out)}, nil
}

// filePath reads the path argument. "~" means root's home, as Cua expanded it for its user.
func filePath(p params) (string, error) {
	path, err := p.mustStr("path")
	if err != nil {
		return "", err
	}
	switch {
	case path == "":
		return "", fmt.Errorf("path is required")
	case path == "~":
		path = homeDir
	case strings.HasPrefix(path, "~/"):
		path = homeDir + path[1:]
	case strings.HasPrefix(path, "-"):
		path = "./" + path // never an option to find, cat or dd
	}
	return path, nil
}

func pathDetail(p params) string { s, _ := p.strOr("path", ""); return clip(s) }

// sh runs script in the desktop with args as $1, $2 and so on.
func sh(ctx context.Context, d desk, stdin []byte, script string, args ...string) ([]byte, error) {
	argv := append([]string{"sh", "-c", script, "sh"}, args...)
	var out []byte
	var err error
	if stdin != nil {
		out, err = d.exec(ctx, bytes.NewReader(stdin), argv...)
	} else {
		out, err = d.exec(ctx, nil, argv...)
	}
	if err != nil {
		return nil, guestErr(err)
	}
	return out, nil
}
