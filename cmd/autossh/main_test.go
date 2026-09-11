//go:build windows

package main

import (
	"bufio"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
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

func TestParseConfigClampsLifetimePolling(t *testing.T) {
	t.Setenv("AUTOSSH_POLL", "600")
	t.Setenv("AUTOSSH_FIRST_POLL", "300")
	t.Setenv("AUTOSSH_MAXLIFETIME", "5")
	c, err := parseConfig([]string{"host"})
	if err != nil {
		t.Fatal(err)
	}
	if c.poll != 5*time.Second || c.firstPoll != 5*time.Second {
		t.Fatalf("poll timers were not clamped: poll=%s first=%s", c.poll, c.firstPoll)
	}
}

func TestMonitorOKLoopMode(t *testing.T) {
	monitor, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close()
	responses, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer responses.Close()
	go func() {
		conn, acceptErr := monitor.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		msg, readErr := bufio.NewReader(conn).ReadBytes('\n')
		if readErr != nil {
			return
		}
		response, dialErr := net.Dial("tcp", responses.Addr().String())
		if dialErr != nil {
			return
		}
		defer response.Close()
		_, _ = response.Write(msg)
	}()
	port := monitor.Addr().(*net.TCPAddr).Port
	l, err := newLogger(config{logLevelSet: true, logLevel: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	if !monitorOK(config{monitorPort: port}, l, responses) {
		t.Fatal("loop monitor probe failed")
	}
}
