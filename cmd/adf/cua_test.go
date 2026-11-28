package main

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8000": true, "localhost:8000": true, "[::1]:8000": true,
		"0.0.0.0:8000": false, ":8000": false, "192.168.1.20:8000": false, "[::]:8000": false, "garbage": false,
	}
	for addr, want := range cases {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestCuaServerNeedsATokenOffLoopback(t *testing.T) {
	t.Setenv("ADF_CUA_TOKEN", "")
	root := newRootCmd()
	root.SetArgs([]string{"cua-server", "--addr", "0.0.0.0:0"})
	root.SetOut(&bytes.Buffer{})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "set a token with --token or ADF_CUA_TOKEN") {
		t.Fatalf("err = %v", err)
	}
}

func TestCuaSetupPrintsBothRecipes(t *testing.T) {
	var out bytes.Buffer
	printCuaSetup(&out, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 18000}, "tok")
	for _, want := range []string{
		"export CUA_API_KEY=tok",
		"export CUA_API_BASE=http://127.0.0.1:18000\n",
		"export CUA_API_BASE_URL=http://127.0.0.1:18000\n",
		"export CUA_BASE_URL=http://127.0.0.1:18000\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("setup does not contain %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	printCuaSetup(&out, &net.TCPAddr{IP: net.IPv4zero, Port: 8000}, "tok")
	if !strings.Contains(out.String(), "CUA_BASE_URL=http://127.0.0.1:8000") || !strings.Contains(out.String(), "this computer's address") {
		t.Errorf("unspecified address:\n%s", out.String())
	}
}
