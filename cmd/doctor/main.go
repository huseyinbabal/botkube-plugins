package main

import (
	"context"
	"fmt"
	"github.com/MakeNowJust/heredoc"
	"github.com/PullRequestInc/go-gpt3"
	"github.com/hashicorp/go-plugin"
	"github.com/kubeshop/botkube/pkg/api"
	"github.com/kubeshop/botkube/pkg/api/executor"
	"github.com/kubeshop/botkube/pkg/pluginx"
	"regexp"
	"strings"
	"sync"
)

const promptTemplate = "Can you show me 3 possible kubectl commands to take an action after resource '%s' fails with error '%s'?"

type Config struct {
	ApiKey *string `yaml:"apiKey,omitempty"`
}

// DoctorExecutor implements the Botkube executor plugin interface.
type DoctorExecutor struct {
	gptClient *gpt3.Client
	l         sync.Mutex
}

func main() {
	executor.Serve(map[string]plugin.Plugin{
		"doctor": &executor.Plugin{
			Executor: &DoctorExecutor{},
		},
	})
}

// Metadata returns details about the Doctor plugin.
func (d *DoctorExecutor) Metadata(context.Context) (api.MetadataOutput, error) {
	return api.MetadataOutput{
		Version:     "1.0.0",
		Description: "Doctor helps in finding the root cause of a k8s problem.",
		JSONSchema: api.JSONSchema{
			Value: heredoc.Doc(`{
       "$schema": "http://json-schema.org/draft-04/schema#",
       "title": "doctor",
       "description": "Doctor helps in finding the root cause of a k8s problem.",
       "type": "object",
       "properties": {
         "apiKey": {
           "description": "Open API Key",
           "type": "string"
         }
       },
       "additionalProperties": false
     }`),
		},
	}, nil
}

// Execute returns a given command as a response.
func (d *DoctorExecutor) Execute(ctx context.Context, in executor.ExecuteInput) (executor.ExecuteOutput, error) {
	var cfg Config
	err := pluginx.MergeExecutorConfigs(in.Configs, &cfg)
	if err != nil {
		return executor.ExecuteOutput{}, fmt.Errorf("while merging input configuration: %w", err)
	}
	doctorParams := normalizeCommand(in.Command)
	gpt := *d.getGptClient(&cfg)
	sb := strings.Builder{}
	err = gpt.CompletionStreamWithEngine(ctx,
		gpt3.TextDavinci003Engine,
		gpt3.CompletionRequest{
			Prompt:      []string{buildPrompt(doctorParams)},
			MaxTokens:   gpt3.IntPtr(300),
			Temperature: gpt3.Float32Ptr(0),
		}, func(resp *gpt3.CompletionResponse) {
			text := resp.Choices[0].Text
			sb.WriteString(text)
		})
	if err != nil {
		return executor.ExecuteOutput{}, err

	}
	response := sb.String()
	response = strings.TrimLeft(response, "\n")
	if doctorParams.IsRaw() {
		return executor.ExecuteOutput{
			Message: api.NewPlaintextMessage(response, true),
		}, nil

	}
	btnBuilder := api.NewMessageButtonBuilder()
	var btns []api.Button
	for _, s := range strings.Split(response, "\n") {
		s = strings.Join(strings.Split(s, "")[3:], "")
		btns = append(btns, btnBuilder.ForCommandWithDescCmd("Choice 1", s, api.ButtonStylePrimary))
	}
	return executor.ExecuteOutput{
		Message: api.Message{
			BaseBody: api.Body{
				Plaintext: "Possible actions",
			},
			Sections: []api.Section{
				{
					Buttons: btns,
				},
			},
			OnlyVisibleForYou: false,
			ReplaceOriginal:   false,
		},
	}, nil
}

// Help returns help message
func (d *DoctorExecutor) Help(context.Context) (api.Message, error) {
	btnBuilder := api.NewMessageButtonBuilder()
	return api.Message{
		Sections: []api.Section{
			{
				Base: api.Base{
					Header:      "Run `doctor` commands",
					Description: "Doctor helps in finding the root cause of a k8s problem.",
				},
				Buttons: []api.Button{
					btnBuilder.ForCommandWithDescCmd("Run", "doctor 'text'"),
				},
			},
		},
	}, nil
}

func (d *DoctorExecutor) getGptClient(cfg *Config) *gpt3.Client {
	d.l.Lock()
	defer d.l.Unlock()
	if d.gptClient == nil {
		c := gpt3.NewClient(*cfg.ApiKey)
		return &c
	}
	return d.gptClient
}

type DoctorParams struct {
	RawText  string
	Resource string
	Error    string
}

func (p *DoctorParams) IsRaw() bool {
	return p.Resource == "" || p.Error == ""
}
func normalizeCommand(command string) DoctorParams {
	pattern := `--(\w+)=([^\s]+)`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindAllStringSubmatch(command, -1)
	params := DoctorParams{}
	params.RawText = command
	for _, match := range matches {
		key := match[1]
		value := match[2]

		switch key {
		case "resource":
			params.Resource = value
		case "error":
			params.Error = value
		}
	}
	return params
}

func buildPrompt(p DoctorParams) string {
	if p.IsRaw() {
		return p.RawText
	}
	return fmt.Sprintf(promptTemplate, p.Resource, p.Error)
}
