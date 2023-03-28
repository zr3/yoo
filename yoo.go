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
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
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

	if len(os.Args) >= 2 && os.Args[1] == "who" {
		showPersonaConfig(yooConfig)
		return
	}

	selectedPersona, titlePersona := determinePersonas(yooConfig)
	systemPrompt, systemFile, err := loadSystemPrompt(os.Getenv("HOME"), selectedPersona)
	check(err, fmt.Sprintf("Could not read the configured system prompt '%s'", systemFile), true)

	userPrompt := getUserPrompt()

	if len(os.Args) >= 2 && os.Args[1] == "--interactive" {
		runInteractiveMode(userPrompt, yooConfig)
		return
	}

	if !config.Bool("quiet") {
		fmt.Println("...")
	}

	client := openai.NewClient(yooConfig.Secrets.OpenAIKey)
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	}

	promptResponse, err := createChatCompletion(client, selectedPersona.Model, messages)
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
		cmd := exec.Command("bat", matches[0])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		check(err, "Error launching less with latest log file", true)
	} else {
		fmt.Println("No log files found.")
	}
}

func showPersonaConfig(yooConfig YooConfig) {
	fmt.Println(yooConfig.DefaultPersona)
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

func runInteractiveMode(initialPrompt string, yooConfig YooConfig) {
	selectedPersona, titlePersona := determinePersonas(yooConfig)
	systemPrompt, systemFile, err := loadSystemPrompt(os.Getenv("HOME"), selectedPersona)
	check(err, fmt.Sprintf("Could not read the configured system prompt '%s'", systemFile), true)
	titleSystemPrompt, titleSystemFile, err := loadSystemPrompt(os.Getenv("HOME"), titlePersona)
	check(err, fmt.Sprintf("Could not read the configured title prompt '%s'", titleSystemFile), true)

	client := openai.NewClient(yooConfig.Secrets.OpenAIKey)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	if initialPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: initialPrompt,
		})
	}

	scanner := bufio.NewScanner(os.Stdin)
	exit := make(chan os.Signal, 1)

	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	for {
		userInput := ""
		select {
		case <-exit:
			fmt.Println("\nExiting...")
			os.Exit(0)
		default:
			fmt.Printf("> ")
			if scanner.Scan() {
				userInput = scanner.Text()
			}

			if userInput == "exit" {
				break
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: userInput,
			})

			response, err := createChatCompletion(client, selectedPersona.Model, messages)
			check(err, "Could not complete request to openai", true)

			fmt.Println(response)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: response,
			})
		}
	}

	userLog := strings.Builder{}
	assistantLog := strings.Builder{}
	for _, message := range messages {
		if message.Role == openai.ChatMessageRoleUser {
			userLog.WriteString(message.Content + "\n")
		} else if message.Role == openai.ChatMessageRoleAssistant {
			assistantLog.WriteString(message.Content + "\n")
		}
	}

	content := sectionDivider("User Messages") + userLog.String() + sectionDivider("Assistant Messages") + assistantLog.String() + sectionDivider("System Message") + systemPrompt
	home := os.Getenv("HOME")
	currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
	titleMessages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: titleSystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userLog.String(),
		},
	}

	title, err := createChatCompletion(client, titlePersona.Model, titleMessages)
	logName := fmt.Sprintf(promptFileFormat, home, currentTime, "interactive-session-"+title)
	os.WriteFile(logName, []byte(content), 0644)
}

func createChatCompletion(client *openai.Client, model string, messages []openai.ChatCompletionMessage) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
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

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: titleSystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: mainContent,
		},
	}
	title, err := createChatCompletion(client, titlePersona.Model, messages)
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
