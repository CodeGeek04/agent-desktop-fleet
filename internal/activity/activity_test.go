package activity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func equal(a, b Event) bool {
	ta, tb := a.Time, b.Time
	a.Time, b.Time = time.Time{}, time.Time{}
	return ta.Equal(tb) && a == b
}

func TestAppendAndReadFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "activity.jsonl")
	l := &Log{Path: path}
	e1 := Event{Time: time.Unix(100, 0), Desktop: "a", Source: SourceAgent, Action: "click", Detail: "(1, 2)", OK: true, DurationMS: 5}
	if err := l.Append(e1); err != nil {
		t.Fatal(err)
	}
	events, cur, err := ReadFrom(path, 0)
	if err != nil || len(events) != 1 || !equal(events[0], e1) {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	e2 := Event{Time: time.Unix(101, 0), Desktop: "b", Source: SourceConsole, Action: "key", Error: "boom"}
	if err := l.Append(e2); err != nil {
		t.Fatal(err)
	}
	events, cur2, _ := ReadFrom(path, cur)
	if len(events) != 1 || !equal(events[0], e2) || cur2 <= cur {
		t.Fatalf("second read: events=%+v cur=%d->%d", events, cur, cur2)
	}
	events, cur3, _ := ReadFrom(path, cur2)
	if len(events) != 0 || cur3 != cur2 {
		t.Fatalf("third read: events=%+v cur=%d", events, cur3)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("log file mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestPartialLineIsLeftForLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "activity.jsonl")
	if err := os.WriteFile(path, []byte(`{"desktop":"a","action":"click","ok":true}`+"\n"+`{"desktop":"b"`), 0o600); err != nil {
		t.Fatal(err)
	}
	events, cur, _ := ReadFrom(path, 0)
	if len(events) != 1 || events[0].Desktop != "a" {
		t.Fatalf("events = %+v", events)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`,"action":"key","ok":true}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	events, _, _ = ReadFrom(path, cur)
	if len(events) != 1 || events[0].Desktop != "b" || events[0].Action != "key" {
		t.Fatalf("events after completing the line = %+v", events)
	}
}

func TestRotationRestartsCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "activity.jsonl")
	l := &Log{Path: path, MaxSize: 200}
	for i := 0; i < 5; i++ {
		if err := l.Append(Event{Desktop: "a", Action: "click", Detail: strings.Repeat("x", 50)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatal("expected a rotated file")
	}
	events, _, _ := ReadFrom(path, 1<<20)
	if len(events) == 0 {
		t.Fatal("a cursor past the end of a rotated file must restart at the beginning")
	}
}

func TestRecentReturnsNewest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "activity.jsonl")
	l := &Log{Path: path}
	for i := 0; i < 10; i++ {
		if err := l.Append(Event{Desktop: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	events, cur, err := Recent(path, 3)
	if err != nil || len(events) != 3 || events[0].Desktop != "7" || events[2].Desktop != "9" {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	fi, _ := os.Stat(path)
	if cur != fi.Size() {
		t.Fatalf("cursor %d, want file size %d", cur, fi.Size())
	}
}

func TestMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.jsonl")
	if events, cur, err := ReadFrom(path, 5); err != nil || events != nil || cur != 0 {
		t.Fatalf("ReadFrom: %v %d %v", events, cur, err)
	}
	if events, cur, err := Recent(path, 5); err != nil || events != nil || cur != 0 {
		t.Fatalf("Recent: %v %d %v", events, cur, err)
	}
}

func TestConcurrentAppendsStayValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "activity.jsonl")
	l := &Log{Path: path}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = l.Append(Event{Desktop: fmt.Sprint(i), Action: "click"})
			}
		}(i)
	}
	wg.Wait()
	events, _, err := ReadFrom(path, 0)
	if err != nil || len(events) != 200 {
		t.Fatalf("got %d events, want 200 (err %v)", len(events), err)
	}
}
