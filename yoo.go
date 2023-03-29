package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
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

type Mode int

const (
	Normal Mode = iota
	Chat
	Config
	Show
)

type LoadedPersona struct {
	Name          string
	Model         string
	SystemMessage openai.ChatCompletionMessage
}

type LoadedResources struct {
	OpenAIClient      *openai.Client
	ChatPersona       LoadedPersona
	TitlePersona      LoadedPersona
	SelectedMode      Mode
	InitialUserPrompt string
	ConfigDir         string
	LogDir            string
}

func main() {
	// read config
	loadedResources := loadConfig()

	switch loadedResources.SelectedMode {
	case Normal:
		normalMode(loadedResources)
	case Chat:
		chatMode(loadedResources)
	case Config:
		configMode()
	case Show:
		showMode()
	default:
		log.Fatal("invalid mode '" + string(loadedResources.SelectedMode) + "'")
	}
}

func normalMode(loadedResources LoadedResources) {
	// if stdin was provided, add that to prompt
	if !termutil.Isatty(os.Stdin.Fd()) {
		inreader := bufio.NewReader(os.Stdin)
		pipedinput, err := ioutil.ReadAll(inreader)
		if err == nil {
			loadedResources.InitialUserPrompt += "\n\n"
			loadedResources.InitialUserPrompt += string(pipedinput)
		}
	}

	// print something for ux
	if !config.Bool("quiet") {
		fmt.Println("...")
	}

	// get the main prompt response
	promptResponse, err := createChatCompletion(
		loadedResources.OpenAIClient,
		loadedResources.ChatPersona,
		loadedResources.InitialUserPrompt,
		[]openai.ChatCompletionMessage{})
	checkError(err, "could not complete request to openai", true)

	// write response out to console
	fmt.Println(promptResponse)

	// log response to file
	title := fetchTitleContent(loadedResources)
	currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
	logName := loadedResources.LogDir + currentTime + "." + title + ".md"
	metaContent := "# " + title + "\n\n" + currentTime
	content := metaContent + div("user") + loadedResources.InitialUserPrompt + div(loadedResources.ChatPersona.Name) + promptResponse + div("system") + loadedResources.ChatPersona.SystemMessage.Content
	os.WriteFile(logName, []byte(content), 0644)
}

func chatMode(loadedResources LoadedResources) {
	fmt.Println("chatting with " + loadedResources.ChatPersona.Name + "!")
	fmt.Println("...")

	history := []openai.ChatCompletionMessage{}
	reader := bufio.NewReader(os.Stdin)
	for {
		// get prompt
		fmt.Print("\n> ")
		userPrompt, err := reader.ReadString('\n')
		checkError(err, "problem reading stdin", true)
		userPrompt = strings.TrimSpace(userPrompt)
		if userPrompt == "quit" || userPrompt == "exit" {
			fmt.Println("chat ended!")
			break
		}

		// get the main prompt response
		promptResponse, err := createChatCompletion(
			loadedResources.OpenAIClient,
			loadedResources.ChatPersona,
			userPrompt,
			history)
		checkError(err, "could not complete request to openai", true)

		// write response out to console
		fmt.Println(promptResponse)

		// add the pair of messages to the history
		history = append(
			history,
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: promptResponse,
			},
		)
	}

	// log response to file
	title := fetchTitleContent(loadedResources)
	currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
	logName := loadedResources.LogDir + currentTime + "." + title + ".md"
	metaContent := "# " + title + "\n\n" + currentTime

	chatHistoryContent := ""
	for _, message := range history {
		chatHistoryContent += string(message.Role) + ":\n" + message.Content + "\n\n"
	}

	content := metaContent + div("chat conversation") + chatHistoryContent + div("system") + loadedResources.ChatPersona.SystemMessage.Content
	os.WriteFile(logName, []byte(content), 0644)
}

func configMode() {
	log.Fatal("config mode is not implemented")
}

func showMode() {
	log.Fatal("show mode is not implemented")
}

func fetchTitleContent(loadedResources LoadedResources) string {
	titleContent := div("system") + loadedResources.ChatPersona.SystemMessage.Content + div("prompt") + loadedResources.InitialUserPrompt
	title, err := createChatCompletion(
		loadedResources.OpenAIClient,
		loadedResources.TitlePersona,
		titleContent,
		[]openai.ChatCompletionMessage{})
	checkError(err, "could not complete request to openai for title slug", false)
	if title == "" {
		return "unknown-topic"
	} else {
		return title
	}
}

func loadConfig() LoadedResources {
	// bind the config file
	home, err := os.UserHomeDir()
	checkError(err, "yoo depends on the home directory for configuration, and no home dir was found.", true)
	config.AddDriver(yamlv3.Driver)
	configDir := home + "/.config/yoo/"
	logDir := home + "/.yoo/"
	configFile := configDir + "config.yml"
	err = config.LoadFiles(configFile)
	checkError(err, "yoo depends on '"+configFile+"' and it was not found.", true)
	yooConfig := YooConfig{}
	err = config.BindStruct("", &yooConfig)
	checkError(err, "yoo had a problem parsing config from '"+configFile+"'", true)

	// load the cli flags
	flags := []string{"persona", "prompt", "quiet:bool", "no-log:bool"}
	err = config.LoadFlags(flags)
	checkError(err, "there was an issue parsing flags, so they will be ignored.", false)

	// set up personas
	chatPersona := Persona{
		Name:  "archie",
		Model: "gpt-4",
	}
	titlePersona := Persona{
		Name:  "slugger",
		Model: "gpt-4",
	}

	// start with configured default persona
	for _, p := range yooConfig.Personas {
		if p.Name == yooConfig.DefaultPersona {
			chatPersona = p
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
				chatPersona = p
			}
		}
	}

	// read the chat system prompt
	chatSystemPrompt, chatSystemFile, err := loadSystemPrompt(home, chatPersona)
	checkError(err, "could not read the configured system prompt '"+chatSystemFile+"'.", true)

	// read the title system prompt
	titleSystemPrompt, titleSystemFile, err := loadSystemPrompt(home, titlePersona)
	checkError(err, "could not read the configured title prompt '"+titleSystemFile+"'.", true)

	// set up an openai client
	openAIClient := openai.NewClient(yooConfig.Secrets.OpenAIKey)

	// determine mode
	selectedMode := Normal // just `yoo`, or `yoo "what is..`
	initialUserPrompt := ""
	if len(os.Args) > 1 {
		if os.Args[1] == "chat" { // `yoo chat`
			selectedMode = Chat
		} else if os.Args[1] == "config" { // `yoo config`
			selectedMode = Config
		} else if os.Args[1] == "show" { // `yoo show`
			selectedMode = Show
		}
		if selectedMode == Normal && os.Args[1][0] != '-' {
			initialUserPrompt = os.Args[1]
		}
	}

	// compose and return usable config
	return LoadedResources{
		OpenAIClient: openAIClient,
		ChatPersona: LoadedPersona{
			Name:  chatPersona.Name,
			Model: chatPersona.Model,
			SystemMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: chatSystemPrompt,
			},
		},
		TitlePersona: LoadedPersona{
			Name:  titlePersona.Name,
			Model: titlePersona.Model,
			SystemMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: titleSystemPrompt,
			},
		},
		SelectedMode:      selectedMode,
		InitialUserPrompt: initialUserPrompt,
		ConfigDir:         configDir,
		LogDir:            logDir,
	}
}

func div(title string) string {
	return "\n\n## " + title + "\n\n"
}

func checkError(err error, message string, isFatal bool) {
	if err != nil {
		fmt.Println(message)
		if isFatal {
			log.Fatal(err)
		} else {
			fmt.Println(err)
		}
	}
}

func createChatCompletion(client *openai.Client, persona LoadedPersona, prompt string, historySlice []openai.ChatCompletionMessage) (string, error) {
	systemSlice := []openai.ChatCompletionMessage{persona.SystemMessage}
	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	}
	fullHistorySlice := append(systemSlice, historySlice...)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    persona.Model,
			Messages: append(fullHistorySlice, userMessage),
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
