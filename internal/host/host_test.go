package host

import "testing"

func TestParseDarwin(t *testing.T) {
	s, err := parseDarwin("25769803776\n58\n{ 4.13 4.23 3.47 }\n", 10)
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalMemBytes != 25769803776 || s.CPUs != 10 || s.Load1 != 4.13 {
		t.Fatalf("stats = %+v", s)
	}
	if want := int64(25769803776) * 58 / 100; s.AvailableMemBytes != want {
		t.Fatalf("available = %d, want %d", s.AvailableMemBytes, want)
	}
}

func TestParseLinux(t *testing.T) {
	meminfo := "MemTotal:       32212254 kB\nMemFree:         1000000 kB\nMemAvailable:   20000000 kB\n"
	s, err := parseLinux(meminfo, "0.52 0.48 0.40 1/234 5678\n", 16)
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalMemBytes != 32212254*1024 || s.AvailableMemBytes != 20000000*1024 || s.CPUs != 16 || s.Load1 != 0.52 {
		t.Fatalf("stats = %+v", s)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := parseDarwin("nonsense", 4); err == nil {
		t.Error("darwin: expected an error")
	}
	if _, err := parseLinux("nonsense", "x", 4); err == nil {
		t.Error("linux: expected an error")
	}
}

func TestReadOnThisMachine(t *testing.T) {
	s, err := Read()
	if err != nil {
		t.Skipf("host stats unavailable here: %v", err)
	}
	if s.TotalMemBytes <= 0 || s.AvailableMemBytes <= 0 || s.AvailableMemBytes > s.TotalMemBytes || s.CPUs <= 0 {
		t.Fatalf("implausible stats %+v", s)
	}
}
