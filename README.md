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
- Diagnostic errors are printed but do not yet produce a nonzero program exit code.
- Provides raw diagnostics, without automatic root-cause analysis.