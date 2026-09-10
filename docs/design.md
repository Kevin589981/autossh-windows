# Windows-native autossh design

This project is an independent implementation, not a Windows build of the
upstream C program. The command-line contract is intentionally compatible with
the useful autossh subset: SSH arguments are passed through, `-M` adds monitor
forwards, and `AUTOSSH_*` settings control retry behavior.

The supervisor is written in Go so the release is a single native PE binary.
`os/exec` starts the local OpenSSH client, a goroutine waits for its exit, and a
timer performs the optional TCP echo probe. `taskkill.exe /T /F` terminates the
SSH process tree when Windows has no POSIX signal equivalent. A startup gate
prevents immediate authentication/configuration errors from becoming an
unbounded loop; later failures are retried with bounded backoff.

The monitor is optional. With `-M 0`, process-exit supervision still works and
users can rely on OpenSSH `ServerAliveInterval` and `ServerAliveCountMax`. With
`-M port`, the loop-forwarding convention is used; with `-M port:echo`, a remote
echo service is used instead.

GitHub Actions runs tests and a Windows build for every push/PR. Version tags
(`v*`) run the same checks and publish the stripped `amd64` executable as a
GitHub Release asset.
