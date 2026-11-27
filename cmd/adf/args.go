package main

import (
	"fmt"
	"strconv"
)

// parseInts converts coordinate arguments to integers.
func parseInts(args ...string) ([]int, error) {
	out := make([]int, len(args))
	for i, s := range args {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", s)
		}
		out[i] = n
	}
	return out, nil
}
