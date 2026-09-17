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

func printSummary(pod, namespace string) {
	output, err := runKubectl(
		"get", "pod", pod,
		"-n", namespace,
		"-o", "json",
	)
	if err != nil {
		fmt.Println("\nSUMMARY")
		fmt.Println("-------")
		fmt.Println("Unable to generate structured summary.")
		return
	}

	var info PodInfo

	if err := json.Unmarshal([]byte(output), &info); err != nil {
		fmt.Println("\nSUMMARY")
		fmt.Println("-------")
		fmt.Println("Unable to parse pod information.")
		return
	}

	fmt.Println("\nSUMMARY")
	fmt.Println("-------")
	fmt.Println("Phase:", info.Status.Phase)

	if len(info.Status.ContainerStatuses) == 0 {
		fmt.Println("No container status available.")
		return
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
		} else if container.State.Running != nil {
			state = "Running"
		} else if container.State.Terminated != nil {
			state = container.State.Terminated.Reason
			lastExitCode = container.State.Terminated.ExitCode
			lastReason = container.State.Terminated.Reason
		}

		if container.LastState.Terminated != nil {
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
		))
	}
}

func suggestionFor(state, lastReason string, restarts int32) string {
	combined := strings.ToLower(state + " " + lastReason)

	switch {
	case strings.Contains(combined, "oomkilled"):
		return "Check container memory usage and memory limits."

	case strings.Contains(combined, "imagepullbackoff"),
		strings.Contains(combined, "errimagepull"):
		return "Check the image name, registry access, and image pull credentials."

	case strings.Contains(combined, "crashloopbackoff"):
		return "Review application logs and startup configuration."

	case strings.Contains(combined, "createcontainerconfigerror"):
		return "Check ConfigMaps, Secrets, environment variables, and volume configuration."

	case strings.Contains(combined, "containercannotrun"):
		return "Check the container command, entrypoint, permissions, and image compatibility."

	case restarts >= 5:
		return "High restart count detected; review current and previous container logs."

	case strings.EqualFold(state, "Running"):
		return "No obvious container failure state detected."

	default:
		return "Review pod events, logs, and pod details for the failure cause."
	}
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

	printSummary(pod, namespace)

	failed := false

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
