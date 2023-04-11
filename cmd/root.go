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
	"context"
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yoo",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yoo.yaml)")

	rootCmd.PersistentFlags().Bool("quiet", false, "hide the CLI ux and only show model output (e.g. for commit message)")
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".yoo" (without extension).
		viper.AddConfigPath(home + "/.config/yoo/")
		viper.SetConfigType("yml")
		viper.SetConfigName("config")
		viper.Set("configpath", home+"/.config/yoo/")
		viper.Set("logpath", home+"/.yoo/")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintln(os.Stderr, "could not load config file")
	}
}

func loadSystemPrompt(persona string) (string, string, error) {
	systemfile := viper.GetString("configpath") + persona + ".txt"
	systemcontent, err := os.ReadFile(systemfile)
	return string(systemcontent), systemfile, err
}

func createChatCompletion(client *openai.Client, persona Persona, prompt string, historySlice []openai.ChatCompletionMessage) (string, error) {
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
