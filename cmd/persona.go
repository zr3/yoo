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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// personaCmd represents the persona command
var personaCmd = &cobra.Command{
	Args:  cobra.MaximumNArgs(1),
	Use:   "persona",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println(viper.GetString("persona"))
			return
		} else {
			persona := args[0]
			// if persona doesn't exist, ask if should create
			// bail if no
			if viper.GetString("personas."+persona)+".model" == "" {
				fmt.Println("persona [" + persona + "] doesn't exist. add it?")
				if !confirmWithUser() {
					return
				}
			}
			// confirm existing model, or add new model
			// model := "gpt-4"
			// open system message in editor w vim fallback
			return
		}
	},
}

func confirmWithUser() bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n≫ ")
		userPrompt, err := reader.ReadString('\n')
		checkError(err, "problem reading stdin", true)
		userPrompt = strings.TrimSpace(userPrompt)
		if userPrompt == "quit" || userPrompt == "exit" || userPrompt == "n" || userPrompt == "N" {
			return false
		} else if userPrompt == "y" || userPrompt == "Y" {
			return true
		} else {
			fmt.Println("please enter 'y' or 'n'")
		}
	}
}
func init() {
	configCmd.AddCommand(personaCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// personaCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// personaCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
