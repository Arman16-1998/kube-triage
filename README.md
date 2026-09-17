# kube-triage

A Go command-line tool that collects and summarizes Kubernetes pod diagnostics using `kubectl`.

## Features

- Pod status and node placement
- Automatic diagnostic summary
- Container readiness and restart counts
- CrashLoopBackOff detection
- OOMKilled detection
- Image pull error detection
- Current and previous container logs, with timestamps
- Pod events ordered by creation time
- Pod details, including exit codes and restart counts
- 30-second timeout for each `kubectl` command
- Exit codes suitable for scripts and automation

## Requirements

- Go matching `go.mod`, or automatic toolchain downloads enabled
- `kubectl` available on `PATH`
- A configured Kubernetes context with permission to read pod diagnostics

## Build

```bash
go build -o kube-triage .
```

## Usage

```bash
./kube-triage <pod> -n <namespace>
```

The namespace defaults to `default`.

Example:

```bash
./kube-triage triage-crash -n default
```

Run directly from source:

```bash
go run . triage-crash -n default
```

## Example summary

```text
SUMMARY
-------
Phase: Running

Container: triage-crash
Ready: false
Restarts: 30
State: CrashLoopBackOff
Last exit code: 1
Last reason: Error
Suggested check: Review application logs and startup configuration.
```

The pod phase and container state are reported separately. For example, Kubernetes may report a pod phase of `Running` while a container is in `CrashLoopBackOff`.

## Diagnostic suggestions

`kube-triage` currently provides basic suggestions for several common failure states:

- `CrashLoopBackOff`
- `OOMKilled`
- `ImagePullBackOff`
- `ErrImagePull`
- `CreateContainerConfigError`
- `ContainerCannotRun`
- High container restart counts

Suggestions are based on Kubernetes state information and are intended to guide investigation, not provide definitive root-cause analysis.

## Command timeout

Every `kubectl` command is limited to 30 seconds.

If the Kubernetes API or a diagnostic command does not respond within that time, `kube-triage` reports a timeout instead of hanging indefinitely.

Example:

```text
ERROR: failed to retrieve pod: kubectl timed out after 30s
```

## Windows and WSL

Tested in Ubuntu on WSL2 using Docker and a local `kind` cluster.

Windows project path:

```text
C:\kube-triage
```

WSL project path:

```text
/mnt/c/kube-triage
```

Run Linux build and test commands inside WSL.

The executable built inside WSL is a Linux binary.

Windows and WSL have separate tool installations and Kubernetes configurations.

## Limitations

- Requires `kubectl`
- Logs are limited to the last 50 lines per container
- Previous container logs may be unavailable
- Historical warnings do not necessarily indicate a current problem
- Pod state may change between diagnostic sections
- Suggestions are heuristic and do not guarantee the root cause
- Multi-container pod selection is not yet supported

## Exit codes

- `0`: Diagnostics collected successfully. This does not mean the pod is healthy.
- `1`: Pod lookup or a required diagnostic command failed.
- `2`: Invalid command arguments.

A failed pod lookup stops the report immediately.

Other required diagnostic failures allow the remaining sections to finish.

Previous logs are optional: failure produces a warning without changing the exit code.

## Help

Use:

```bash
./kube-triage --help
```

to display usage.

## WSL build troubleshooting

If building fails with:

```text
error obtaining VCS status
```

use:

```bash
go build -buildvcs=false -o kube-triage .
```

This disables Git metadata embedding in the executable.