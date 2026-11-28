package cuaserver

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
)

// params are a command's arguments as decoded with json.Decoder.UseNumber. Cua's server
// drops arguments a handler does not take, so unknown keys are ignored here too, and a
// JSON null counts as absent (cua-computer sends {"x": null} for "the current position").
type params map[string]any

func (p params) has(key string) bool { return p[key] != nil }

func badParam(key, want string, v any) error {
	return fmt.Errorf("%s must be %s, got %v", key, want, v)
}

// number reads a JSON number. Numeric strings are accepted too, since some agents quote them.
func (p params) number(key string) (float64, bool, error) {
	v := p[key]
	switch n := v.(type) {
	case nil:
		return 0, false, nil
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false, badParam(key, "a number", v)
		}
		return f, true, nil
	case float64:
		return n, true, nil
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f, true, nil
		}
	}
	return 0, false, badParam(key, "a number", v)
}

// int reads a whole number, rounding fractions: models often send coordinates like 640.0.
func (p params) int(key string) (int, bool, error) {
	f, ok, err := p.number(key)
	if !ok || err != nil {
		return 0, ok, err
	}
	if math.IsNaN(f) || math.Abs(f) > 1e9 {
		return 0, false, badParam(key, "a reasonable number", p[key])
	}
	return int(math.Round(f)), true, nil
}

// intOr reads an optional whole number with a default.
func (p params) intOr(key string, def int) (int, error) {
	n, ok, err := p.int(key)
	if !ok {
		return def, err
	}
	return n, err
}

func (p params) mustInt(key string) (int, error) {
	n, ok, err := p.int(key)
	if err == nil && !ok {
		err = fmt.Errorf("%s is required", key)
	}
	return n, err
}

func (p params) str(key string) (string, bool, error) {
	switch v := p[key].(type) {
	case nil:
		return "", false, nil
	case string:
		return v, true, nil
	default:
		return "", false, badParam(key, "a string", v)
	}
}

func (p params) strOr(key, def string) (string, error) {
	s, ok, err := p.str(key)
	if !ok {
		return def, err
	}
	return s, err
}

func (p params) mustStr(key string) (string, error) {
	s, ok, err := p.str(key)
	if err == nil && !ok {
		err = fmt.Errorf("%s is required", key)
	}
	return s, err
}

func (p params) boolOr(key string, def bool) (bool, error) {
	switch v := p[key].(type) {
	case nil:
		return def, nil
	case bool:
		return v, nil
	default:
		return false, badParam(key, "true or false", v)
	}
}

// strings reads a list of strings, such as hotkey's keys.
func (p params) strings(key string) ([]string, error) {
	list, ok := p[key].([]any)
	if !ok {
		if p[key] == nil {
			return nil, fmt.Errorf("%s is required", key)
		}
		return nil, badParam(key, "a list of strings", p[key])
	}
	out := make([]string, len(list))
	for i, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, badParam(key, "a list of strings", p[key])
		}
		out[i] = s
	}
	return out, nil
}

// point reads optional x and y. Both absent means the current pointer position (nil).
func (p params) point() (*computer.Point, error) {
	x, okX, err := p.int("x")
	if err != nil {
		return nil, err
	}
	y, okY, err := p.int("y")
	if err != nil {
		return nil, err
	}
	switch {
	case okX && okY:
		return &computer.Point{X: x, Y: y}, nil
	case okX || okY:
		return nil, fmt.Errorf("give both x and y, or neither to use the current pointer position")
	}
	return nil, nil
}

func (p params) button() (computer.Button, error) {
	b, err := p.strOr("button", "left")
	return computer.Button(b), err
}

// path reads a drag path: a list of [x, y] pairs, or of {"x": X, "y": Y} objects.
func (p params) path(key string) ([]computer.Point, error) {
	list, ok := p[key].([]any)
	if !ok {
		if p[key] == nil {
			return nil, fmt.Errorf("%s is required", key)
		}
		return nil, badParam(key, "a list of [x, y] points", p[key])
	}
	out := make([]computer.Point, 0, len(list))
	for i, item := range list {
		var xy params
		switch v := item.(type) {
		case []any:
			if len(v) != 2 {
				return nil, fmt.Errorf("%s[%d] must be an [x, y] pair", key, i)
			}
			xy = params{"x": v[0], "y": v[1]}
		case map[string]any:
			xy = params(v)
		default:
			return nil, fmt.Errorf("%s[%d] must be an [x, y] pair", key, i)
		}
		x, errX := xy.mustInt("x")
		y, errY := xy.mustInt("y")
		if errX != nil || errY != nil {
			return nil, fmt.Errorf("%s[%d] must be an [x, y] pair of numbers", key, i)
		}
		out = append(out, computer.Point{X: x, Y: y})
	}
	return out, nil
}
