# kube-triage

A Go command-line tool that collects and summarizes Kubernetes pod diagnostics using `kubectl`.

## Features

- Pod status and node placement
- Structured diagnostic summary
- Container readiness and restart counts
- Current and latest recorded termination details
- Suggestions for common container failure states
- Current and previous container logs, with timestamps
- Pod events ordered by creation time
- Detailed pod information
- Report export while keeping output visible in the terminal
- Protection against overwriting existing report files
- 30-second timeout for each `kubectl` command
- Exit codes suitable for scripts and automation

## Requirements

- Go matching `go.mod`, or automatic toolchain downloads enabled
- `kubectl` available on `PATH`
- A configured Kubernetes context with permission to read pods, events, and pod logs

Run the tool in the same environment where `kubectl` can access your cluster.

## Build

```bash
go build -o kube-triage .
```

## Usage

```bash
./kube-triage <pod> [-n <namespace>] [--output <file>]
```

The namespace defaults to `default`. Both `-n` and `--namespace` are supported after the pod name.

Example:

```bash
./kube-triage triage-crash -n default
```

Run directly from source:

```bash
go run . triage-crash -n default
```

## Save a report

Use `--output` to save diagnostics while also displaying them in the terminal:

```bash
./kube-triage triage-crash -n default --output report.txt
```

The file includes diagnostic output, warnings, and errors.

- Relative paths are resolved from your current directory.
- The destination directory must already exist.
- Existing files are never overwritten.
- Choose a new filename for each additional report.
- Report creation or writing failures return exit code `1`.
- Failed diagnostic runs can still save a partial report.
- “Report saved” confirms the file was written; check the exit code to determine whether diagnostic collection succeeded.

If the file already exists, the tool stops before collecting diagnostics:

```text
ERROR: cannot create report file: open report.txt: file exists
```

The example file `report.txt` at the repository root is excluded from Git. Reports saved under other names are not automatically ignored.

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

Pod phase and container state are reported separately. A pod can have phase `Running` while a container is in `CrashLoopBackOff`.

When a container is currently terminated, its current termination details take precedence over an older termination. Otherwise, the summary uses the previous termination details when available.

## Diagnostic suggestions

The tool provides basic suggestions for:

- `CrashLoopBackOff`
- `OOMKilled`
- `ImagePullBackOff`
- `ErrImagePull`
- `CreateContainerConfigError`
- `ContainerCannotRun`
- Running containers that are not ready
- Completed containers
- High restart counts of five or more

Suggestions are based on reported container state and termination history. They guide investigation and do not establish a definitive root cause.

The summary currently covers regular containers only.

## Command timeout

Each `kubectl` command has a 30-second execution timeout.

The timeout applies separately to each command, not to the entire report. Diagnostic commands run sequentially, so a complete report can take longer than 30 seconds.

The process runner also allows up to two seconds for output-pipe cleanup when needed.

Example:

```text
ERROR: failed to retrieve pod: kubectl timed out after 30s
```

A timeout during the initial pod lookup stops the report. Timeouts in later required diagnostics mark the run as failed while allowing the remaining sections to run.

Previous container logs are optional.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Required diagnostics collected and output written successfully. This does not mean the pod is healthy. |
| `1` | Pod lookup, summary retrieval/parsing, a required diagnostic command, or report creation/writing failed. |
| `2` | Invalid command arguments. |

A failed pod lookup stops the report immediately.

A summary failure or another required diagnostic failure allows the remaining sections to finish, then returns exit code `1`.

If the previous-log command returns an error, the tool prints a warning without changing the exit code.

A pod with no regular container statuses yet produces an informational summary message, rather than a summary error.

## Help

```bash
./kube-triage --help
```

The `-h` option is also supported.

## Windows and WSL

Tested inside Ubuntu on WSL2 using Docker and a local `kind` cluster.

The same project folder is accessible at:

- Windows: `C:\kube-triage`
- Ubuntu on WSL: `/mnt/c/kube-triage`

Run the Linux build and usage commands inside WSL. An executable built there is a Linux binary.

Windows and WSL have separate tool installations and Kubernetes configurations.

After changing `main.go`, rebuild the executable before running it.

If you export `report.txt` while working in `/mnt/c/kube-triage`, the same file is available in Windows at `C:\kube-triage\report.txt`.

## WSL build troubleshooting

If building fails with:

```text
error obtaining VCS status
```

build with:

```bash
go build -buildvcs=false -o kube-triage .
```

This disables Git metadata embedding in the executable.

For running directly from source with the same workaround:

```bash
go run -buildvcs=false . triage-crash -n default
```

## Limitations

- Requires `kubectl`; does not call the Kubernetes API directly
- Logs are limited to the last 50 lines per container
- Previous container logs may be unavailable
- Historical warning events do not necessarily indicate a current problem
- Pod state may change between diagnostic sections
- Suggestions do not guarantee the root cause
- Summarizes regular containers and requests their logs; selecting one container is not yet supported
- Init containers and ephemeral containers are not included in the summary
- Successful diagnostic collection does not mean the workload is healthy
- Command failure detection relies on the exit status returned by `kubectl`; error-like text returned with exit status `0` is displayed without being classified as a failure
- Report export uses plain text; JSON and HTML export are not supported