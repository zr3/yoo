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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// peepCmd represents the peep command
var peepCmd = &cobra.Command{
	Use:   "peep",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// todo: search, list
		// for now: return latest
		logPath := viper.GetString("logpath")
		var latestFile os.FileInfo
		var latestTime time.Time

		err := filepath.Walk(logPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// If not a directory and file modification time is greater
			// than the current latest time, update the latest file
			if !info.IsDir() && info.ModTime().After(latestTime) {
				latestFile = info
				latestTime = info.ModTime()
			}

			return nil
		})

		if err != nil {
			log.Fatal("Error walking the directory:", err)
			return
		}

		if latestFile != nil {
			fmt.Println(logPath + latestFile.Name())
		} else {
			log.Fatal("No files found in the directory.")
		}
	},
}

func init() {
	rootCmd.AddCommand(peepCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// peepCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// peepCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
