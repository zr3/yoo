package cmd

import (
	openai "github.com/sashabaranov/go-openai"
)

type Persona struct {
	Name          string
	Model         string
	SystemMessage openai.ChatCompletionMessage
}

type LoadedResources struct {
	OpenAIClient *openai.Client
	ChatPersona  Persona
	TitlePersona Persona
	LogDir       string
}
