package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runKubectl(args ...string) string {
	cmd := exec.Command("kubectl", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("ERROR: %v\n%s", err, string(output))
	}

	return string(output)
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

	status := runKubectl(
		"get", "pod", pod,
		"-n", namespace,
		"-o", "wide",
	)
	fmt.Println(status)

	fmt.Println("\nPOD EVENTS")
	fmt.Println("----------")

	events := runKubectl(
		"get", "events",
		"-n", namespace,
		"--field-selector", "involvedObject.kind=Pod,involvedObject.name="+pod,
		"--sort-by=.metadata.creationTimestamp",
	)
	fmt.Println(events)

	fmt.Println("\nPOD LOGS")
	fmt.Println("--------")

	logs := runKubectl(
		"logs", pod,
		"-n", namespace,
		"--all-containers=true",
		"--tail=50",
		"--timestamps=true",
	)
	fmt.Println(logs)

	fmt.Println("\nPREVIOUS CONTAINER LOGS")
	fmt.Println("-----------------------")

	previousLogs := runKubectl(
		"logs", pod,
		"-n", namespace,
		"--all-containers=true",
		"--previous",
		"--tail=50",
		"--timestamps=true",
	)
	fmt.Println(previousLogs)

	fmt.Println("\nPOD DETAILS")
	fmt.Println("-----------")

	details := runKubectl(
		"describe", "pod", pod,
		"-n", namespace,
	)
	fmt.Println(details)
}
