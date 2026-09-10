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
