# autossh-windows

Windows-native SSH connection supervisor. It starts `ssh.exe`, watches the
child process, optionally probes an SSH TCP forwarding monitor, and restarts
the session after an unexpected exit or a failed probe. The executable has no
WSL, MSYS2, Cygwin, or runtime dependency.

## Build

Install Go 1.24 or newer, then run in PowerShell:

```powershell
./build.ps1
```

The result is `autossh.exe`. To cross-build from another OS:

```text
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" ./cmd/autossh
```

## Usage

```text
autossh [-V] [-f] [-M monitor[:echo]] [SSH_OPTIONS] [user@]host
```

For a forwarding monitor, `-M 20000` adds `-L 20000:127.0.0.1:20000` and
`-R 20000:127.0.0.1:20001` to the SSH command. `-M 20000:7` uses a remote
echo service and only adds the local forward. `-M 0` disables TCP monitoring;
process-exit supervision remains active.

## Compatibility with upstream autossh

This is an independent Windows-native implementation. It follows the useful
upstream command-line and retry conventions, but it is not a source-compatible
port of the Unix C program.

| Feature | Upstream autossh | autossh-windows |
| --- | --- | --- |
| SSH child process | `fork`/`exec` | Native Windows `ssh.exe` child process |
| `-M 0` | Disable TCP monitor; restart on SSH exit | Same behavior |
| `-M port` | Two-port loop monitor (`port` plus `port+1`) | Same two-port loop monitor |
| `-M port:echo` | Remote echo-service monitor | Same single-connection echo monitor |
| SSH exit restart | Restart unexpected exits | Restart exit codes 1, 2, and 255 |
| Startup gate | `AUTOSSH_GATETIME` | Supported |
| Retry backoff | Bounded quadratic backoff | Bounded quadratic backoff |
| Maximum starts | `AUTOSSH_MAXSTART` | Supported |
| Maximum lifetime | `AUTOSSH_MAXLIFETIME` | Supported; polling is clamped to the remaining lifetime |
| SSH executable | `AUTOSSH_PATH` | Supported; defaults to `ssh.exe` from `PATH` |
| Monitoring interval | `AUTOSSH_POLL`, `AUTOSSH_FIRST_POLL` | Supported |
| Log file/level | `AUTOSSH_LOGFILE`, `AUTOSSH_LOGLEVEL`, `AUTOSSH_DEBUG` | Supported; writes stderr or a file instead of syslog |
| PID file | `AUTOSSH_PIDFILE` | Supported |
| Echo message | `AUTOSSH_MESSAGE` | Supported (maximum 64 bytes) |
| `AUTOSSH_PORT` | Overrides `-M` | Same behavior |
| `-f` | Detach with Unix daemonization | Re-launches as a detached Windows process; use Task Scheduler or a Windows service for boot/startup integration |
| Manual signal restart | `SIGUSR1`/Unix signals | Not available; stop/restart the Windows process instead |
| Service integration | Cygwin `AUTOSSH_NTSERVICE` | Not implemented; use a native Windows service wrapper |
| Process termination | POSIX `SIGTERM` and `waitpid` | `taskkill.exe /T /F` terminates the SSH process tree |
| Logging backend | syslog or file | stderr or file; Windows has no built-in syslog contract |
| Build/install | autoconf + make | Go + PowerShell; single native PE executable |
| Release artifacts | Depends on packaging system | GitHub Actions publishes `autossh-windows-amd64.exe` on `v*` tags |

### Windows-specific behavior

- Install or enable **OpenSSH Client** so that `ssh.exe` is available in
  `PATH`, or set `AUTOSSH_PATH` to an absolute path.
- `-f` re-launches the supervisor as a detached process and redirects its
  console streams to `NUL`. Set `AUTOSSH_LOGFILE` when detached logs are needed.
  For unattended startup use Task Scheduler, NSSM/WinSW, or another service
  manager and run `autossh.exe` directly.
- Ctrl+C and process termination stop the supervisor and its SSH child tree.
  Unix-only signals such as `SIGUSR1` are not exposed by Windows.
- The monitor binds loopback addresses only (`127.0.0.1`). With `-M port`,
  the response listener is `port+1`; ensure both local ports are available.
- Child cleanup uses `taskkill /T /F`, so a forced restart can be less graceful
  than a Unix `SIGTERM`.
- Release binaries currently target Windows amd64. Go can cross-build other
  Windows architectures, but they are not attached automatically to releases.
- Starting `autossh.exe` without arguments prints the usage text and exits
  successfully. This makes the portable executable discoverable to package
  managers; a real supervisor run still requires an SSH destination.

The implementation follows the established autossh startup-gate and restart
semantics while using Windows process management (`taskkill /T /F`) for child
trees. `-f` is accepted for command-line compatibility; Windows services,
Task Scheduler, or `Start-Process` should provide detachment.

Supported environment variables: `AUTOSSH_PATH`, `AUTOSSH_PORT`,
`AUTOSSH_POLL`, `AUTOSSH_FIRST_POLL`, `AUTOSSH_GATETIME`, `AUTOSSH_MAXSTART`,
`AUTOSSH_MAXLIFETIME`, `AUTOSSH_LOGFILE`, `AUTOSSH_LOGLEVEL`, `AUTOSSH_DEBUG`,
`AUTOSSH_MESSAGE`, and `AUTOSSH_PIDFILE`.

## Release

Push a tag such as `v1.0.0`. GitHub Actions runs tests, builds a stripped
`autossh-windows-amd64.exe`, and attaches it to a GitHub Release automatically.

## WinGet

The planned WinGet publication shape and the official submission checklist are
documented in [docs/winget.md](docs/winget.md). This project has not been
submitted to the WinGet Community Repository yet.
