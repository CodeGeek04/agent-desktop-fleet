package runner

import (
	"context"
	"io"
)

// Call is one recorded Fake.Run invocation.
type Call struct {
	Name  string
	Args  []string
	Stdin []byte
}

// Result is one scripted Fake.Run outcome.
type Result struct {
	Stdout []byte
	Err    error
}

// Fake records calls and replays Results in order. Once Results is empty it returns (nil, nil).
type Fake struct {
	Calls   []Call
	Results []Result
}

// Run implements Runner.
func (f *Fake) Run(_ context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	c := Call{Name: name, Args: append([]string(nil), args...)}
	if stdin != nil {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		c.Stdin = b
	}
	f.Calls = append(f.Calls, c)
	if len(f.Results) == 0 {
		return nil, nil
	}
	r := f.Results[0]
	f.Results = f.Results[1:]
	return r.Stdout, r.Err
}
