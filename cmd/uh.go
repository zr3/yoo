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
	"time"

	termutil "github.com/andrew-d/go-termutil"
	"github.com/briandowns/spinner"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// uhCmd represents the uh command
var uhCmd = &cobra.Command{
	Use:   "uh",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MaximumNArgs(1),
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
		s := spinner.New(spinner.CharSets[19], 100*time.Millisecond)
		if !viper.GetBool("quiet") {
			fmt.Println("asking " + chatPersona.Name + "!")
			s.Color("cyan")
			s.Prefix = "╰─ "
			s.Start()
		}

		// set up an openai client
		openAIClient := openai.NewClient(viper.GetString("secrets.openai-key"))

		// get the main prompt response
		promptResponse, err := createChatCompletion(
			openAIClient,
			chatPersona,
			userPrompt,
			[]openai.ChatCompletionMessage{})
		checkError(err, "could not complete request to openai", true)

		// write response out to console
		if s.Active() {
			s.Stop()
		}
		fmt.Println("╰─ " + promptResponse)

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
		content := metaContent + div("user") + userPrompt + div(chatPersona.Name) + promptResponse + div("system") + chatPersona.SystemMessage.Content
		os.WriteFile(logName, []byte(content), 0644)
	},
}

func init() {
	rootCmd.AddCommand(uhCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// uhCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// uhCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
