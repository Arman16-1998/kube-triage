# kube-triage

A Go command-line tool that collects Kubernetes pod diagnostics using kubectl.

## Features

- Pod status and node placement
- Pod events ordered by creation time
- Current and previous container logs, with timestamps
- Pod details, including exit codes and restart counts

## Requirements

- Go matching go.mod, or automatic toolchain downloads enabled
- kubectl available on PATH
- A configured Kubernetes context with permission to read diagnostics

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

## Windows and WSL

Tested in Ubuntu on WSL2 using Docker and a local kind cluster.

Windows project path: `C:\kube-triage`
WSL project path: `/mnt/c/kube-triage`

Run these commands inside WSL. The executable built there is a Linux binary.
Windows and WSL have separate tool installations and Kubernetes configurations.

## Limitations

- Requires kubectl.
- Logs are limited to the last 50 lines per container.
- Previous container logs may be unavailable.
- Historical warnings do not necessarily indicate a current problem.
- Pod state may change between diagnostic sections.
- Provides raw diagnostics, without automatic root-cause analysis.


## Exit codes

- `0`: Diagnostics collected successfully. This does not mean the pod is healthy.
- `1`: Pod lookup or a required diagnostic command failed.
- `2`: Invalid command arguments.

A failed pod lookup stops the report immediately.
Other required diagnostic failures allow the remaining sections to finish.
Previous logs are optional: failure produces a warning without changing the exit code.

## Help

Use `./kube-triage --help` to display usage.

## WSL build troubleshooting

If building fails with "error obtaining VCS status", use:

    go build -buildvcs=false -o kube-triage .

This disables Git metadata embedding in the executable.