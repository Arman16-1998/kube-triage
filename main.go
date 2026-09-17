package main

import (
	"fmt"
	"os"
	"os/exec"
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
	if len(os.Args) < 2 {
		fmt.Println("Usage: kube-triage <pod> -n <namespace>")
		return
	}

	pod := os.Args[1]
	namespace := "default"

	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "-n" && i+1 < len(os.Args) {
			namespace = os.Args[i+1]
		}
	}

	fmt.Println("KUBE TRIAGE")
	fmt.Println("===========")
	fmt.Println("Pod:", pod)
	fmt.Println("Namespace:", namespace)

	fmt.Println("\nPOD STATUS")
	fmt.Println("----------")

	status := runKubectl(
		"get",
		"pod",
		pod,
		"-n",
		namespace,
		"-o",
		"wide",
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
