//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "1.4g-windows"

type config struct {
	sshPath     string
	sshArgs     []string
	monitorPort int
	echoPort    int
	monitor     bool
	background  bool
	poll        time.Duration
	firstPoll   time.Duration
	gate        time.Duration
	maxStarts   int
	maxLifetime time.Duration
	message     string
	pidFile     string
	logLevel    int
	logLevelSet bool
	logFile     string
}

type logger struct {
	mu sync.Mutex
	*log.Logger
	level int
	file  *os.File
}

func (l *logger) printf(level int, format string, args ...any) {
	if level > l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Printf(format, args...)
}

func (l *logger) close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if errors.Is(err, errVersion) {
		fmt.Printf("autossh %s\n", version)
		return
	}
	if errors.Is(err, errHelp) {
		usage(os.Stdout)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "autossh:", err)
		usage(os.Stderr)
		os.Exit(2)
	}

	l, err := newLogger(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "autossh:", err)
		os.Exit(1)
	}
	defer l.close()
	if cfg.pidFile != "" {
		if err := os.WriteFile(cfg.pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
			l.printf(0, "cannot write pid file %q: %v", cfg.pidFile, err)
			os.Exit(1)
		}
		defer os.Remove(cfg.pidFile)
	}
	if cfg.background {
		l.printf(1, "-f is not supported as a detached console operation on Windows; use Start-Process or a service manager")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	code := run(ctx, cfg, l)
	os.Exit(code)
}

var (
	errVersion = errors.New("version")
	errHelp    = errors.New("help")
)

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: autossh [-V] [-f] [-M monitor[:echo]] [SSH_OPTIONS] [user@]host")
	fmt.Fprintln(w, "Windows-native autossh supervisor. SSH options are passed through to ssh.exe.")
	fmt.Fprintln(w, "  -M port[:echo]  enable the TCP monitor (port 0 disables it)")
	fmt.Fprintln(w, "  -f              compatibility flag; use a Windows service/task for detaching")
	fmt.Fprintln(w, "  -V              print version")
	fmt.Fprintln(w, "Environment: AUTOSSH_PATH, AUTOSSH_PORT, AUTOSSH_POLL, AUTOSSH_FIRST_POLL,")
	fmt.Fprintln(w, "  AUTOSSH_GATETIME, AUTOSSH_MAXSTART, AUTOSSH_MAXLIFETIME, AUTOSSH_LOGFILE,")
	fmt.Fprintln(w, "  AUTOSSH_LOGLEVEL, AUTOSSH_DEBUG, AUTOSSH_MESSAGE, AUTOSSH_PIDFILE")
}

func parseConfig(args []string) (config, error) {
	c := config{sshPath: "ssh.exe", poll: 10 * time.Minute, firstPoll: 10 * time.Minute, gate: 30 * time.Second, maxStarts: -1}
	if v := os.Getenv("AUTOSSH_PATH"); v != "" {
		c.sshPath = v
	}
	if v := os.Getenv("AUTOSSH_POLL"); v != "" {
		d, err := seconds(v)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("invalid AUTOSSH_POLL %q", v)
		}
		c.poll = d
	}
	c.firstPoll = c.poll
	if v := os.Getenv("AUTOSSH_FIRST_POLL"); v != "" {
		d, err := seconds(v)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("invalid AUTOSSH_FIRST_POLL %q", v)
		}
		c.firstPoll = d
	}
	if v := os.Getenv("AUTOSSH_GATETIME"); v != "" {
		d, err := seconds(v)
		if err != nil || d < 0 {
			return c, fmt.Errorf("invalid AUTOSSH_GATETIME %q", v)
		}
		c.gate = d
	}
	if v := os.Getenv("AUTOSSH_MAXSTART"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < -1 {
			return c, fmt.Errorf("invalid AUTOSSH_MAXSTART %q", v)
		}
		c.maxStarts = n
	}
	if v := os.Getenv("AUTOSSH_MAXLIFETIME"); v != "" {
		d, err := seconds(v)
		if err != nil || d < 0 {
			return c, fmt.Errorf("invalid AUTOSSH_MAXLIFETIME %q", v)
		}
		c.maxLifetime = d
	}
	if v := os.Getenv("AUTOSSH_MESSAGE"); len(v) > 64 {
		return c, fmt.Errorf("AUTOSSH_MESSAGE exceeds 64 bytes")
	} else if v != "" {
		c.message = v
	}
	if v := os.Getenv("AUTOSSH_PIDFILE"); v != "" {
		c.pidFile = v
	}
	if v := os.Getenv("AUTOSSH_LOGFILE"); v != "" {
		c.logFile = v
	}
	if v := os.Getenv("AUTOSSH_LOGLEVEL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 7 {
			return c, fmt.Errorf("invalid AUTOSSH_LOGLEVEL %q", v)
		}
		c.logLevel = n
		c.logLevelSet = true
	}
	if os.Getenv("AUTOSSH_DEBUG") != "" {
		c.logLevel = 7
		c.logLevelSet = true
	}
	if v := os.Getenv("AUTOSSH_PORT"); v != "" {
		p, e, err := parseMonitor(v)
		if err != nil {
			return c, err
		}
		c.monitorPort, c.echoPort, c.monitor = p, e, p != 0
	}

	ssh := make([]string, 0, len(args)+5)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-V", "--version":
			return c, errVersion
		case "-h", "--help":
			return c, errHelp
		case "-f":
			c.background = true
			continue
		case "-M":
			if i+1 >= len(args) {
				return c, errors.New("-M requires a port")
			}
			i++
			p, e, err := parseMonitor(args[i])
			if err != nil {
				return c, err
			}
			if os.Getenv("AUTOSSH_PORT") == "" {
				c.monitorPort, c.echoPort, c.monitor = p, e, p != 0
			}
			continue
		case "--":
			ssh = append(ssh, args[i+1:]...)
			i = len(args)
			continue
		}
		if strings.HasPrefix(a, "-M") && len(a) > 2 {
			p, e, err := parseMonitor(a[2:])
			if err != nil {
				return c, err
			}
			if os.Getenv("AUTOSSH_PORT") == "" {
				c.monitorPort, c.echoPort, c.monitor = p, e, p != 0
			}
			continue
		}
		ssh = append(ssh, a)
	}
	if len(ssh) == 0 {
		return c, errors.New("an SSH destination is required")
	}
	if c.monitor && c.monitorPort > 65534 {
		return c, errors.New("monitor port must be between 0 and 65534")
	}
	if c.monitor {
		ssh = addMonitorForwards(ssh, c.monitorPort, c.echoPort)
	}
	c.sshArgs = ssh
	return c, nil
}

func seconds(v string) (time.Duration, error) {
	n, err := strconv.ParseInt(v, 10, 64)
	return time.Duration(n) * time.Second, err
}

func parseMonitor(v string) (int, int, error) {
	parts := strings.Split(v, ":")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid monitor port %q", v)
	}
	p, err := strconv.Atoi(parts[0])
	if err != nil || p < 0 || p > 65535 {
		return 0, 0, fmt.Errorf("invalid monitor port %q", v)
	}
	e := 0
	if len(parts) == 2 {
		e, err = strconv.Atoi(parts[1])
		if err != nil || e < 1 || e > 65535 {
			return 0, 0, fmt.Errorf("invalid echo port %q", parts[1])
		}
	}
	return p, e, nil
}

func addMonitorForwards(args []string, port, echo int) []string {
	forwards := []string{"-L", fmt.Sprintf("%d:127.0.0.1:%d", port, func() int {
		if echo != 0 {
			return echo
		}
		return port
	}())}
	if echo == 0 {
		forwards = append(forwards, "-R", fmt.Sprintf("%d:127.0.0.1:%d", port, port+1))
	}
	result := make([]string, 0, len(args)+len(forwards))
	result = append(result, forwards...)
	result = append(result, args...)
	return result
}

func newLogger(c config) (*logger, error) {
	level := c.logLevel
	if !c.logLevelSet {
		level = 6
	}
	w := io.Writer(os.Stderr)
	l := &logger{Logger: log.New(w, "", log.LstdFlags), level: level}
	if c.logFile != "" {
		f, err := os.OpenFile(c.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		l.file = f
		l.Logger.SetOutput(f)
	}
	return l, nil
}

func run(ctx context.Context, c config, l *logger) int {
	startedAt := time.Now()
	starts := 0
	fastFailures := 0
	var lastStart time.Time
	for c.maxStarts < 0 || starts < c.maxStarts {
		if c.maxLifetime > 0 && time.Since(startedAt) >= c.maxLifetime {
			l.printf(1, "maximum lifetime reached")
			return 0
		}
		if !lastStart.IsZero() {
			fastFailures = backoff(ctx, c.poll, fastFailures, lastStart, l)
			if fastFailures < 0 {
				return 1
			}
		}
		starts++
		lastStart = time.Now()
		l.printf(1, "starting ssh (count %d)", starts)
		cmd := exec.Command(c.sshPath, c.sshArgs...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			l.printf(0, "cannot start %q: %v", c.sshPath, err)
			return 1
		}
		l.printf(2, "ssh child pid is %d", cmd.Process.Pid)
		result := watch(ctx, c, l, cmd, starts == 1, startTime, startedAt)
		if result == exitOK {
			return 0
		}
		if result == exitErr {
			return 1
		}
	}
	l.printf(1, "maximum start count reached; exiting")
	return 0
}

type watchResult int

const (
	restart watchResult = iota
	exitOK
	exitErr
)

func watch(ctx context.Context, c config, l *logger, cmd *exec.Cmd, first bool, startTime, lifetime time.Time) watchResult {
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	interval := c.firstPoll
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			killTree(cmd, l)
			return exitErr
		case err := <-waitCh:
			code := exitCode(err)
			if first && c.gate > 0 && time.Since(startTime) <= c.gate {
				l.printf(0, "ssh exited prematurely with status %d", code)
				return exitErr
			}
			switch code {
			case 0:
				l.printf(1, "ssh exited with status 0; exiting")
				return exitOK
			case 1, 2, 255:
				l.printf(1, "ssh exited with status %d; restarting", code)
				return restart
			default:
				l.printf(1, "ssh exited with status %d; exiting", code)
				return exitErr
			}
		case <-timer.C:
			if c.maxLifetime > 0 && time.Since(lifetime) >= c.maxLifetime {
				killTree(cmd, l)
				return exitOK
			}
			if c.monitor && !monitorOK(c, l) {
				l.printf(1, "monitor check failed; restarting ssh")
				killTree(cmd, l)
				return restart
			}
			timer.Reset(c.poll)
		}
	}
}

func (c *config) _unused() {}

func monitorOK(c config, l *logger) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(c.monitorPort))
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		l.printf(3, "monitor connect %s: %v", addr, err)
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	msg := fmt.Sprintf("%s autossh %d %d %s\r\n", os.Getenv("COMPUTERNAME"), os.Getpid(), time.Now().UnixNano(), c.message)
	if _, err = conn.Write([]byte(msg)); err != nil {
		return false
	}
	buf := make([]byte, len(msg))
	_, err = io.ReadFull(conn, buf)
	return err == nil && string(buf) == msg
}

func backoff(ctx context.Context, poll time.Duration, failures int, lastStart time.Time, l *logger) int {
	if time.Since(lastStart) >= maxDuration(poll/10, 10*time.Second) {
		return 0
	}
	failures++
	if failures <= 5 {
		return failures
	}
	d := time.Duration(float64(poll) / 100.0 * float64(failures-5) * float64(failures-5) / 3)
	if d > poll {
		d = poll
	}
	l.printf(2, "waiting %s before restart", d)
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return -1
	case <-t.C:
		return failures
	}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func killTree(cmd *exec.Cmd, l *logger) {
	if cmd.Process == nil {
		return
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	k := exec.Command("taskkill.exe", "/PID", pid, "/T", "/F")
	if out, err := k.CombinedOutput(); err != nil {
		l.printf(2, "taskkill %s failed: %v (%s)", pid, err, strings.TrimSpace(string(out)))
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() >= 0 {
			return ee.ExitCode()
		}
	}
	return 255
}
