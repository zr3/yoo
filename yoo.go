package main

import (
	"bufio"
	"context"
	"fmt"
	termutil "github.com/andrew-d/go-termutil"
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/yamlv3"
	openai "github.com/sashabaranov/go-openai"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type YooConfig struct {
	Secrets        Secrets `mapstructure:"secrets"`
	DefaultPersona string  `mapstructure:"default-persona"`
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
	if err != nil {
		panic(err)
	}
	config.AddDriver(yamlv3.Driver)
	yooconfig := YooConfig{}
	err = config.LoadFiles(home + "/.config/yoo/config.yml")
	if err != nil {
		panic(err)
	}
	err = config.BindStruct("", &yooconfig)
	
	flags := []string{"persona", "prompt", "quiet:bool"}
	err = config.LoadFlags(flags)

	// default to archie gpt-4 persona
	persona := Persona{
		Name: "archie",
		Model: "gpt-4",
	}
	// override with configured default persona, if exists
	for _, p := range yooconfig.Personas {
		if p.Name == persona.Name {
			persona = p
		}
	}
	// override with flagged persona, if provided
	personaFlag := config.String("persona")
	if personaFlag != "" {
		for _, p := range yooconfig.Personas {
			if p.Name == personaFlag {
				persona = p
			}
		}
	}

	// fmt.Println("config read. using " + systemfile + " as system prompt.")

	// build system + prompt request
	systemfile := home + "/.config/yoo/" + persona.Name + ".txt"
	systemcontent, err := os.ReadFile(systemfile)
	if err != nil {
		log.Fatal(err)
	}
	system := string(systemcontent)

	// start prompt with cli argument
	prompt := ""
	if len(os.Args) == 2 {
		prompt += os.Args[1]
	} else if len(os.Args) > 2 {
		prompt += config.String("prompt")
	}

	// if stdin was provided, add that to prompt
	if !termutil.Isatty(os.Stdin.Fd()) {
		inreader := bufio.NewReader(os.Stdin)
		pipedinput, err := ioutil.ReadAll(inreader)
		if err == nil {
			prompt += "\n\n"
			prompt += string(pipedinput)
		}
	}

	if !config.Bool("quiet") {
		fmt.Println("...")
	}

	// make request to openai to get response
	client := openai.NewClient(yooconfig.Secrets.OpenAIKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			// Model: openai.GPT3Dot5Turbo,
			Model: persona.Model,
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
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}
	promptresponse := resp.Choices[0].Message.Content

	// write out to console
	fmt.Println(promptresponse)

	// write out to log
	title := "latest"
	currenttime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
	logname := home + "/.yoo/" + currenttime + "." + title + ".md"
	content := "# " + title + div("prompt") + prompt + div("response") + promptresponse + div("system") + system

	os.WriteFile(logname, []byte(content), 0666)
}

func div(title string) string {
	return "\n\n## " + title + "\n\n"
}

