package main

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/go-plugin"
	"github.com/huseyinbabal/botkube-plugins/api"
	botkubeplugin "github.com/huseyinbabal/botkube-plugins/api/executor"
	"os/exec"
	"strings"
)

type KubectlExecutor struct{}

func (KubectlExecutor) Execute(command string) (string, error) {
	return run(command)
}

func run(command string) (string, error) {
	commandParts := strings.Split(command, " ")
	cmd := exec.Command(commandParts[0], commandParts[1:]...)

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("cmd: %s, err: %s", command, errOut.String())
	}
	return out.String(), nil
}

func main() {
	api.Serve(map[string]plugin.Plugin{
		"kubectl": &botkubeplugin.ExecutorPlugin{Impl: &KubectlExecutor{}},
	})
}
