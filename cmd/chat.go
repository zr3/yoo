/*
Copyright © 2023 Zak Reynolds <zak.reynolds@zakjr.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	termutil "github.com/andrew-d/go-termutil"
	"github.com/briandowns/spinner"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// start with a blank user prompt
		userPrompt := ""
		if len(args) > 0 {
			userPrompt += args[0]
		}

		// if stdin was provided, add that to prompt
		if !termutil.Isatty(os.Stdin.Fd()) {
			inreader := bufio.NewReader(os.Stdin)
			pipedinput, err := io.ReadAll(inreader)
			if err == nil {
				userPrompt += "\n\n"
				userPrompt += string(pipedinput)
			}
		}

		// set up personas
		chatPersonaName := viper.GetString("persona")
		chatSystemPrompt, file, err := loadSystemPrompt(chatPersonaName)
		checkError(err, "system prompt file could not be read: "+file, true)
		chatPersona := Persona{
			chatPersonaName,
			viper.GetString("personas." + chatPersonaName + ".model"),
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: chatSystemPrompt,
			},
		}

		// print something for ux
		if !viper.GetBool("quiet") {
			fmt.Println("chatting with " + chatPersona.Name + "!")
		}

		// set up an openai client
		openAIClient := openai.NewClient(viper.GetString("secrets.openai-key"))

		// todo: modularize above
		// loop
		history := []openai.ChatCompletionMessage{}
		reader := bufio.NewReader(os.Stdin)
		s := spinner.New(spinner.CharSets[19], 100*time.Millisecond)
		s.Prefix = "╰─ "
		for {
			// get prompt
			fmt.Print("\n≫ ")
			userPrompt, err := reader.ReadString('\n')
			checkError(err, "problem reading stdin", true)
			userPrompt = strings.TrimSpace(userPrompt)
			if userPrompt == "quit" || userPrompt == "exit" {
				fmt.Println("chat ended!")
				break
			}

			// get the prompt response
			s.Color("cyan")
			s.Start()
			promptResponse, err := createChatCompletion(
				openAIClient,
				chatPersona,
				userPrompt,
				history)
			checkError(err, "could not complete request to openai", true)
			s.Stop()

			// write response out to console
			fmt.Println("╰─ " + promptResponse)

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
		// todo: modularize below

		// set up title persona
		titlePersonaName := viper.GetString("title-persona")
		titleSystemPrompt, file, err := loadSystemPrompt(titlePersonaName)
		checkError(err, "system prompt file could not be read: "+file, true)
		titlePersona := Persona{
			titlePersonaName,
			viper.GetString("personas." + titlePersonaName + ".model"),
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: titleSystemPrompt,
			},
		}

		// get title content
		titleContent := div("system") + chatPersona.SystemMessage.Content + div("prompt") + userPrompt
		title, err := createChatCompletion(
			openAIClient,
			titlePersona,
			titleContent,
			[]openai.ChatCompletionMessage{})
		checkError(err, "could not complete request to openai for title slug", false)
		if title == "" {
			title = "unknown-topic"
		}

		// log response to file
		currentTime := time.Now().Local().Format("2006-01-02--15-04-05-MST")
		logName := viper.GetString("logpath") + currentTime + "." + title + ".md"
		metaContent := "# " + title + "\n\n" + currentTime

		chatHistoryContent := ""
		for _, message := range history {
			chatHistoryContent += string(message.Role) + ":\n" + message.Content + "\n\n"
		}

		content := metaContent + div("chat conversation") + chatHistoryContent + div("system") + chatPersona.SystemMessage.Content
		os.WriteFile(logName, []byte(content), 0644)
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// chatCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// chatCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
