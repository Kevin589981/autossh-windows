//go:build windows

package main

import (
	"os"
	"reflect"
	"testing"
)

func TestParseMonitor(t *testing.T) {
	for _, tc := range []struct {
		in   string
		p, e int
	}{{"20000", 20000, 0}, {"20000:7", 20000, 7}, {"0", 0, 0}} {
		p, e, err := parseMonitor(tc.in)
		if err != nil || p != tc.p || e != tc.e {
			t.Fatalf("parseMonitor(%q) = %d:%d,%v", tc.in, p, e, err)
		}
	}
}

func TestAddMonitorForwards(t *testing.T) {
	got := addMonitorForwards([]string{"-N", "host"}, 20000, 0)
	want := []string{"-L", "20000:127.0.0.1:20000", "-R", "20000:127.0.0.1:20001", "-N", "host"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseConfigStripsOwnFlags(t *testing.T) {
	os.Unsetenv("AUTOSSH_PORT")
	c, err := parseConfig([]string{"-M", "20000:7", "-N", "-o", "ServerAliveInterval=10", "host"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.sshArgs, []string{"-L", "20000:127.0.0.1:7", "-N", "-o", "ServerAliveInterval=10", "host"}) {
		t.Fatalf("ssh args: %#v", c.sshArgs)
	}
}
