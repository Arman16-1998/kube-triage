package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const commandTimeout = 30 * time.Second

type PodInfo struct {
	Status struct {
		Phase             string `json:"phase"`
		ContainerStatuses []struct {
			Name         string `json:"name"`
			Ready        bool   `json:"ready"`
			RestartCount int32  `json:"restartCount"`
			State        struct {
				Waiting *struct {
					Reason string `json:"reason"`
				} `json:"waiting"`
				Running    *struct{} `json:"running"`
				Terminated *struct {
					Reason   string `json:"reason"`
					ExitCode int32  `json:"exitCode"`
				} `json:"terminated"`
			} `json:"state"`
			LastState struct {
				Terminated *struct {
					Reason   string `json:"reason"`
					ExitCode int32  `json:"exitCode"`
				} `json:"terminated"`
			} `json:"lastState"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

func runKubectl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf(
			"kubectl timed out after %s", commandTimeout,
		)
	}

	return string(output), err
}

func printSummary(pod, namespace string) error {
	fmt.Println("\nSUMMARY")
	fmt.Println("-------")

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

	fmt.Println("Phase:", info.Status.Phase)

	if len(info.Status.ContainerStatuses) == 0 {
		fmt.Println("No container status available yet; review pod events.")
		return nil
	}

	for _, container := range info.Status.ContainerStatuses {
		fmt.Println("\nContainer:", container.Name)
		fmt.Println("Ready:", container.Ready)
		fmt.Println("Restarts:", container.RestartCount)

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

		fmt.Println("State:", state)

		if lastExitCode >= 0 {
			fmt.Println("Last exit code:", lastExitCode)
		}
		if lastReason != "" {
			fmt.Println("Last reason:", lastReason)
		}

		fmt.Println("Suggested check:", suggestionFor(
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

func main() {
	usage := "Usage: kube-triage <pod> [-n <namespace>]"

	failUsage := func(message string) {
		fmt.Fprintln(os.Stderr, message)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	if len(os.Args) == 2 &&
		(os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println(usage)
		return
	}

	if len(os.Args) < 2 {
		failUsage("Error: pod name is required.")
	}

	pod := os.Args[1]
	namespace := "default"

	if strings.TrimSpace(pod) == "" || strings.HasPrefix(pod, "-") {
		failUsage("Error: provide a pod name before any options.")
	}

	namespaceSet := false

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-n", "--namespace":
			if namespaceSet {
				failUsage("Error: namespace was specified more than once.")
			}
			if i+1 >= len(os.Args) {
				failUsage("Error: namespace option requires a value.")
			}

			i++
			namespace = os.Args[i]

			if strings.TrimSpace(namespace) == "" ||
				strings.HasPrefix(namespace, "-") {
				failUsage("Error: namespace option requires a value.")
			}

			namespaceSet = true

		default:
			failUsage("Error: unexpected argument: " + os.Args[i])
		}
	}

	fmt.Println("KUBE TRIAGE")
	fmt.Println("===========")
	fmt.Println("Pod:", pod)
	fmt.Println("Namespace:", namespace)

	fmt.Println("\nPOD STATUS")
	fmt.Println("----------")

	status, err := runKubectl(
		"get", "pod", pod,
		"-n", namespace,
		"-o", "wide",
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: failed to retrieve pod:", err)
		if status != "" {
			fmt.Fprintln(os.Stderr, strings.TrimSpace(status))
		}
		os.Exit(1)
	}

	fmt.Println(status)

	failed := false

	if err := printSummary(pod, namespace); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: summary failed:", err)
		failed = true
	}

	printSection := func(title string, optional bool, args ...string) {
		fmt.Println("\n" + title)
		fmt.Println(strings.Repeat("-", len(title)))

		output, err := runKubectl(args...)
		if err != nil {
			if optional {
				fmt.Fprintln(os.Stderr,
					"WARNING: optional diagnostic unavailable:", err)
			} else {
				fmt.Fprintln(os.Stderr,
					"ERROR: diagnostic collection failed:", err)
				failed = true
			}

			if output != "" {
				fmt.Fprintln(os.Stderr, strings.TrimSpace(output))
			}
			return
		}

		fmt.Println(output)
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
		os.Exit(1)
	}
}
