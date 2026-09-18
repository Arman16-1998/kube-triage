package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const commandTimeout = 30 * time.Second

type termination struct {
	Reason   string `json:"reason"`
	ExitCode int32  `json:"exitCode"`
}

type containerState struct {
	Waiting *struct {
		Reason string `json:"reason"`
	} `json:"waiting"`
	Running    *struct{}    `json:"running"`
	Terminated *termination `json:"terminated"`
}

type containerStatus struct {
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	RestartCount int32          `json:"restartCount"`
	State        containerState `json:"state"`
	LastState    containerState `json:"lastState"`
}

type PodInfo struct {
	Status struct {
		Phase             string            `json:"phase"`
		ContainerStatuses []containerStatus `json:"containerStatuses"`
	} `json:"status"`
}

// reportWriter sends each message to the terminal and, optionally, a file.
// It remembers write errors so an incomplete export cannot report success.
type reportWriter struct {
	file *os.File
	err  error
}

func (r *reportWriter) write(terminal io.Writer, values ...any) {
	message := fmt.Sprintln(values...)

	if _, err := io.WriteString(terminal, message); err != nil && r.err == nil {
		r.err = err
	}

	if r.file != nil {
		if _, err := io.WriteString(r.file, message); err != nil && r.err == nil {
			r.err = err
		}
	}
}

func (r *reportWriter) println(values ...any) {
	r.write(os.Stdout, values...)
}

func (r *reportWriter) errorln(values ...any) {
	r.write(os.Stderr, values...)
}

func runKubectl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.WaitDelay = 2 * time.Second

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf(
			"kubectl timed out after %s", commandTimeout,
		)
	}

	return string(output), err
}

func printSummary(report *reportWriter, pod, namespace string) error {
	report.println("\nSUMMARY")
	report.println("-------")

	output, err := runKubectl(
		"get", "pod", pod,
		"-n", namespace,
		"-o", "json",
	)
	if err != nil {
		return fmt.Errorf(
			"retrieve pod information: %w\n%s",
			err, strings.TrimSpace(output),
		)
	}

	var info PodInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return fmt.Errorf("parse pod information: %w", err)
	}

	report.println("Phase:", info.Status.Phase)

	if len(info.Status.ContainerStatuses) == 0 {
		report.println("No container status available yet; review pod events.")
		return nil
	}

	for _, container := range info.Status.ContainerStatuses {
		report.println("\nContainer:", container.Name)
		report.println("Ready:", container.Ready)
		report.println("Restarts:", container.RestartCount)

		state := "Unknown"
		lastExitCode := int32(-1)
		lastReason := ""

		if container.State.Waiting != nil {
			state = container.State.Waiting.Reason
			if state == "" {
				state = "Waiting"
			}
		} else if container.State.Running != nil {
			state = "Running"
		} else if container.State.Terminated != nil {
			state = container.State.Terminated.Reason
			if state == "" {
				state = "Terminated"
			}
			lastExitCode = container.State.Terminated.ExitCode
			lastReason = container.State.Terminated.Reason
		}

		if container.State.Terminated == nil &&
			container.LastState.Terminated != nil {
			lastExitCode = container.LastState.Terminated.ExitCode
			lastReason = container.LastState.Terminated.Reason
		}

		report.println("State:", state)

		if lastExitCode >= 0 {
			report.println("Last exit code:", lastExitCode)
		}
		if lastReason != "" {
			report.println("Last reason:", lastReason)
		}

		report.println("Suggested check:", suggestionFor(
			state,
			lastReason,
			container.RestartCount,
			container.Ready,
		))
	}

	return nil
}

func suggestionFor(state, lastReason string, restarts int32, ready bool) string {
	switch strings.ToLower(state) {
	case "imagepullbackoff", "errimagepull":
		return "Check the image name, registry access, and image pull credentials."

	case "createcontainerconfigerror":
		return "Check ConfigMaps, Secrets, environment variables, and volume configuration."

	case "containercannotrun":
		return "Check the container command, entrypoint, permissions, and image compatibility."

	case "oomkilled":
		return "Check container memory usage, memory limits, and node memory pressure."

	case "crashloopbackoff":
		if strings.EqualFold(lastReason, "OOMKilled") {
			return "Last termination was OOMKilled; check memory usage, limits, and node memory pressure."
		}
		return "Review application logs and startup configuration."

	case "completed":
		return "Container completed; confirm this is expected for the workload."
	}

	if strings.EqualFold(state, "Running") && !ready {
		return "Container is running but not ready; check readiness and startup probes, logs, and dependencies."
	}

	if strings.EqualFold(lastReason, "OOMKilled") {
		return "A recorded termination was OOMKilled; review memory usage, limits, and node memory pressure."
	}

	if restarts >= 5 {
		return "High restart count detected; review current and previous container logs."
	}

	if strings.EqualFold(state, "Running") && ready {
		return "Container is running and ready; review any historical restarts separately."
	}

	return "Review pod events, logs, and pod details to investigate the container state."
}

func collectReport(report *reportWriter, pod, namespace string) int {
	report.println("KUBE TRIAGE")
	report.println("===========")
	report.println("Pod:", pod)
	report.println("Namespace:", namespace)

	report.println("\nPOD STATUS")
	report.println("----------")

	status, err := runKubectl(
		"get", "pod", pod,
		"-n", namespace,
		"-o", "wide",
	)
	if err != nil {
		report.errorln("ERROR: failed to retrieve pod:", err)
		if status != "" {
			report.errorln(strings.TrimSpace(status))
		}
		return 1
	}

	report.println(status)

	failed := false

	if err := printSummary(report, pod, namespace); err != nil {
		report.errorln("ERROR: summary failed:", err)
		failed = true
	}

	printSection := func(title string, optional bool, args ...string) {
		report.println("\n" + title)
		report.println(strings.Repeat("-", len(title)))

		output, err := runKubectl(args...)
		if err != nil {
			if optional {
				report.errorln(
					"WARNING: optional diagnostic unavailable:", err,
				)
			} else {
				report.errorln(
					"ERROR: diagnostic collection failed:", err,
				)
				failed = true
			}

			if output != "" {
				report.errorln(strings.TrimSpace(output))
			}
			return
		}

		report.println(output)
	}

	printSection(
		"POD EVENTS", false,
		"get", "events",
		"-n", namespace,
		"--field-selector", "involvedObject.kind=Pod,involvedObject.name="+pod,
		"--sort-by=.metadata.creationTimestamp",
	)

	printSection(
		"POD LOGS", false,
		"logs", pod,
		"-n", namespace,
		"--all-containers=true",
		"--tail=50",
		"--timestamps=true",
	)

	printSection(
		"PREVIOUS CONTAINER LOGS", true,
		"logs", pod,
		"-n", namespace,
		"--all-containers=true",
		"--previous",
		"--tail=50",
		"--timestamps=true",
	)

	printSection(
		"POD DETAILS", false,
		"describe", "pod", pod,
		"-n", namespace,
	)

	if failed {
		return 1
	}
	return 0
}

func run(args []string) int {
	usage := "Usage: kube-triage <pod> [-n <namespace>] [--output <file>]"

	failUsage := func(message string) int {
		fmt.Fprintln(os.Stderr, message)
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println(usage)
		return 0
	}

	if len(args) == 0 {
		return failUsage("Error: pod name is required.")
	}

	pod := args[0]
	namespace := "default"
	outputPath := ""

	if strings.TrimSpace(pod) == "" || strings.HasPrefix(pod, "-") {
		return failUsage("Error: provide a pod name before any options.")
	}

	namespaceSet := false
	outputSet := false

	for i := 1; i < len(args); i++ {
		option := args[i]

		switch option {
		case "-n", "--namespace", "--output":
			if i+1 >= len(args) {
				return failUsage("Error: " + option + " requires a value.")
			}

			i++
			value := args[i]

			if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "-") {
				return failUsage("Error: " + option + " requires a value.")
			}

			if option == "--output" {
				if outputSet {
					return failUsage("Error: output was specified more than once.")
				}
				outputPath = value
				outputSet = true
			} else {
				if namespaceSet {
					return failUsage("Error: namespace was specified more than once.")
				}
				namespace = value
				namespaceSet = true
			}

		default:
			return failUsage("Error: unexpected argument: " + option)
		}
	}

	report := &reportWriter{}

	if outputSet {
		file, err := os.OpenFile(
			outputPath,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0600,
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: cannot create report file:", err)
			return 1
		}
		report.file = file
	}

	exitCode := collectReport(report, pod, namespace)

	if report.file != nil {
		if err := report.file.Close(); err != nil && report.err == nil {
			report.err = err
		}
	}

	if report.err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: could not fully write report:", report.err)
		return 1
	}

	if outputSet {
		fmt.Fprintln(os.Stderr, "Report saved to:", outputPath)
	}

	return exitCode
}

func main() {
	os.Exit(run(os.Args[1:]))
}
