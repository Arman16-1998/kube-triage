package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runKubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
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
