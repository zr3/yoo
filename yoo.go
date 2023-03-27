package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	termutil "github.com/andrew-d/go-termutil"
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/yamlv3"
	openai "github.com/sashabaranov/go-openai"
)

type YooConfig struct {
	Secrets        Secrets   `mapstructure:"secrets"`
	DefaultPersona string    `mapstructure:"default-persona"`
	TitlePersona   string    `mapstructure:"title-persona"`
	Personas       []Persona `mapstructure:"personas"`
}

type Secrets struct {
	OpenAIKey string `mapstructure:"openai-key"`
}

type Persona struct {
	Name string `mapstructure:"name"`
	// https://github.com/sashabaranov/go-openai/blob/2ebb265e715ad6fc6c1f755705dfbd02d44a9111/completion.go
	Model string `mapstructure:"model"`
}

func main() {
	// read config
	home, err := os.UserHomeDir()
	checkFatal(err, "yoo depends on the home directory for configuration, and no home dir was found.")

	config.AddDriver(yamlv3.Driver)
	yooConfig := YooConfig{}
	configFile := home + "/.config/yoo/config.yml"
	err = config.LoadFiles(configFile)
	checkFatal(err, "yoo depends on '"+configFile+"' and it was not found.")
	err = config.BindStruct("", &yooConfig)
	checkFatal(err, "yoo had a problem parsing config from '"+configFile+"'")

	flags := []string{"persona", "prompt", "quiet:bool", "no-log:bool"}
	err = config.LoadFlags(flags)
	checkWarn(err, "there was an issue parsing flags, so they will be ignored.")

	// default to archie gpt-4 mainPersona
	mainPersona := Persona{
		Name:  "archie",
		Model: "gpt-4",
	}

	// default title persona
	titlePersona := Persona{
		Name:  "summer-slug",
		Model: "gpt-4",
	}

	// override with configured default persona, if exists
	for _, p := range yooConfig.Personas {
		if p.Name == yooConfig.DefaultPersona {
			mainPersona = p
		}
		if p.Name == yooConfig.TitlePersona {
			titlePersona = p
		}
	}

	// override with flagged persona, if provided
	personaFlag := config.String("persona")
	if personaFlag != "" {
		for _, p := range yooConfig.Personas {
			if p.Name == personaFlag {
				mainPersona = p
			}
		}
	}

	// read the systemPrompt prompt
	systemPrompt, systemFile, err := loadSystemPrompt(home, mainPersona)
	checkFatal(err, "could not read the configured system prompt '"+systemFile+"'.")

	// start userPrompt with cli argument
	userPrompt := ""
	if len(os.Args) == 2 {
		userPrompt += os.Args[1]
	} else if len(os.Args) > 2 {
		userPrompt += config.String("prompt")
	}

	// if stdin was provided, add that to prompt
	if !termutil.Isatty(os.Stdin.Fd()) {
		inreader := bufio.NewReader(os.Stdin)
		pipedinput, err := ioutil.ReadAll(inreader)
		if err == nil {
			userPrompt += "\n\n"
			userPrompt += string(pipedinput)
		}
	}

	// print something for ux
	if !config.Bool("quiet") {
		fmt.Println("...")
	}

	// set up an openai client
	client := openai.NewClient(yooConfig.Secrets.OpenAIKey)

	// get the main prompt response
	promptResponse, err := createChatCompletion(client, mainPersona.Model, systemPrompt, userPrompt)
	checkFatal(err, "could not complete request to openai")

	// write response out to console
	fmt.Println(promptResponse)

	// write response out to log
	if !config.Bool("no-log") {
		mainContent := div("prompt") + userPrompt + div("response") + promptResponse + div("system") + systemPrompt
		titleSystemPrompt, titleSystemPromptFile, err := loadSystemPrompt(home, titlePersona)
		checkFatal(err, "could not read the configured title system prompt '"+titleSystemPromptFile+"'.")
		title, err := createChatCompletion(client, titlePersona.Model, titleSystemPrompt, mainContent)
		checkWarn(err, "could not complete request to openai for title slug")
		if err != nil {
			title = "unknown-topic"
		}
		currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
		logName := home + "/.yoo/" + currentTime + "." + title + ".md"
		metaContent := "# " + title + "\n\n" + currentTime
		content := metaContent + mainContent
		os.WriteFile(logName, []byte(content), 0644)
	}
}

func div(title string) string {
	return "\n\n## " + title + "\n\n"
}

func checkFatal(err error, message string) {
	if err != nil {
		fmt.Println(message)
		log.Fatal(err)
	}
}

func checkWarn(err error, message string) {
	if err != nil {
		fmt.Println(message)
		fmt.Println(err)
	}
}

func createChatCompletion(client *openai.Client, model string, system string, prompt string) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: system,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

func loadSystemPrompt(home string, persona Persona) (string, string, error) {
	systemfile := home + "/.config/yoo/" + persona.Name + ".txt"
	systemcontent, err := os.ReadFile(systemfile)
	return string(systemcontent), systemfile, err
}
