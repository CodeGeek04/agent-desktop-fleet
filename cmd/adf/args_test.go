package main

import "testing"

func TestParseInts(t *testing.T) {
	got, err := parseInts("10", "20", "30")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("got %v", got)
	}
	if _, err := parseInts("10", "x"); err == nil {
		t.Fatal("expected an error for a non-integer")
	}
}
