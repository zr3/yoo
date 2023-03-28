package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andrew-d/go-termutil"
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
	Name  string `mapstructure:"name"`
	Model string `mapstructure:"model"`
}

const (
	configFileFormat   = "%s/.config/yoo/config.yml"
	promptFileFormat   = "%s/.yoo/%s.%s.md"
	systemPromptFormat = "%s/.config/yoo/%s.txt"
	defaultPersonaName = "archie"
	defaultModel       = "gpt-4"
	defaultTitleName   = "summer-slug"
	defaultTitleModel  = "gpt-3.5-turbo"
)

func main() {
	yooConfig, err := loadYooConfig()
	check(err, "Error loading YooConfig", true)

	if len(os.Args) >= 2 && os.Args[1] == "config" {
		os.Args = os.Args[1:]

		persona := getConfigValue("persona", yooConfig.DefaultPersona, false)
		model := getConfigValue("model", defaultModel, false)
		system := getConfigValue("system", "", true)

		setPersonaConfig(persona, model, system)
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "latest" {
		launchLatestLogFile()
		return
	}

	selectedPersona, titlePersona := determinePersonas(yooConfig)
	systemPrompt, systemFile, err := loadSystemPrompt(os.Getenv("HOME"), selectedPersona)
	check(err, fmt.Sprintf("Could not read the configured system prompt '%s'", systemFile), true)

	userPrompt := getUserPrompt()

	if !config.Bool("quiet") {
		fmt.Println("...")
	}

	client := openai.NewClient(yooConfig.Secrets.OpenAIKey)
	promptResponse, err := createChatCompletion(client, selectedPersona.Model, systemPrompt, userPrompt)
	check(err, "Could not complete request to openai", true)

	fmt.Println(promptResponse)

	if !config.Bool("no-log") {
		home := os.Getenv("HOME")
		logResponse(home, titlePersona, userPrompt, systemPrompt, promptResponse, client)
	}
}

func getConfigValue(key, defaultValue string, allowEmpty bool) string {
	value := config.String(key)

	if value == "" && !allowEmpty {
		return defaultValue
	}
	return value
}

func setPersonaConfig(persona, model, system string) {
	home := os.Getenv("HOME")

	// Update system prompt
	if system != "" {
		systemFile := fmt.Sprintf(systemPromptFormat, home, persona)
		ioutil.WriteFile(systemFile, []byte(system), 0644)
	}

	// Update model
	if model != "" {
		configFilePath := fmt.Sprintf(configFileFormat, home)
		configContentBytes, err := ioutil.ReadFile(configFilePath)
		check(err, "Error reading the config file for updating persona", true)
		configContent := string(configContentBytes)

		updatedConfigContent := strings.ReplaceAll(configContent, fmt.Sprintf("model: %s", persona), fmt.Sprintf("model: %s", model))
		ioutil.WriteFile(configFilePath, []byte(updatedConfigContent), 0644)
	}
}

func launchLatestLogFile() {
	home := os.Getenv("HOME")
	matches, err := filepath.Glob(fmt.Sprintf("%s/.yoo/*.*.md", home))
	check(err, "Error listing log files", true)

	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	if len(matches) > 0 {
		cmd := exec.Command("less", matches[0])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		check(err, "Error launching less with latest log file", true)
	} else {
		fmt.Println("No log files found.")
	}
}

func loadYooConfig() (YooConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return YooConfig{}, fmt.Errorf("yoo depends on the home directory for configuration, and no home dir was found: %w", err)
	}

	configFilePath := fmt.Sprintf(configFileFormat, home)
	config.AddDriver(yamlv3.Driver)
	yooConfig := YooConfig{}

	err = config.LoadFiles(configFilePath)
	if err != nil {
		return YooConfig{}, fmt.Errorf("yoo depends on '%s', and it was not found: %w", configFilePath, err)
	}

	err = config.BindStruct("", &yooConfig)
	if err != nil {
		return YooConfig{}, fmt.Errorf("yoo had a problem parsing config from '%s': %w", configFilePath, err)
	}

	flags := []string{"persona", "model", "system", "prompt", "quiet:bool", "no-log:bool"}
	err = config.LoadFlags(flags)
	if err != nil {
		check(err, "There was an issue parsing flags, so they will be ignored.", false)
	}

	return yooConfig, nil
}

func determinePersonas(yooConfig YooConfig) (selectedPersona, titlePersona Persona) {
	selectedPersona = Persona{
		Name:  defaultPersonaName,
		Model: defaultModel,
	}

	titlePersona = Persona{
		Name:  defaultTitleName,
		Model: defaultTitleModel,
	}

	for _, p := range yooConfig.Personas {
		if p.Name == yooConfig.DefaultPersona {
			selectedPersona = p
		}
		if p.Name == yooConfig.TitlePersona {
			titlePersona = p
		}
	}

	personaFlag := config.String("persona")
	if personaFlag != "" {
		for _, p := range yooConfig.Personas {
			if p.Name == personaFlag {
				selectedPersona = p
			}
		}
	}

	return selectedPersona, titlePersona
}

func getUserPrompt() string {
	userPrompt := ""
	if len(os.Args) >= 3 {
		if !strings.HasPrefix(os.Args[1], "--") {
			userPrompt += os.Args[1]
		} else {
			userPrompt += config.String("prompt")
		}
	}

	if !termutil.Isatty(os.Stdin.Fd()) {
		inreader := bufio.NewReader(os.Stdin)
		pipedinput, err := ioutil.ReadAll(inreader)
		if err == nil {
			userPrompt += "\n\n"
			userPrompt += string(pipedinput)
		}
	}
	return userPrompt
}

func check(err error, message string, fatal bool) {
	if err != nil {
		fmt.Println(message)
		if fatal {
			log.Fatal(err)
		} else {
			fmt.Println(err)
		}
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
	systemfile := fmt.Sprintf(systemPromptFormat, home, persona.Name)
	systemcontent, err := os.ReadFile(systemfile)
	return string(systemcontent), systemfile, err
}

func sectionDivider(title string) string {
	sanitizedTitle := url.QueryEscape(title)
	return fmt.Sprintf("\n\n## %s\n\n", sanitizedTitle)
}

func logResponse(home string, titlePersona Persona, userPrompt, systemPrompt, promptResponse string, client *openai.Client) {
	mainContent := sectionDivider("prompt") + userPrompt + sectionDivider("response") + promptResponse + sectionDivider("system") + systemPrompt
	titleSystemPrompt, titleSystemPromptFile, err := loadSystemPrompt(home, titlePersona)
	check(err, fmt.Sprintf("Could not read the configured title system prompt '%s'", titleSystemPromptFile), false)

	title, err := createChatCompletion(client, titlePersona.Model, titleSystemPrompt, mainContent)
	check(err, "Could not complete request to openai for title slug", false)

	if err != nil {
		title = "unknown-topic"
	}
	currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
	logName := fmt.Sprintf(promptFileFormat, home, currentTime, title)
	metaContent := "# " + title + "\n\n" + currentTime
	content := metaContent + mainContent
	os.WriteFile(logName, []byte(content), 0644)
}
