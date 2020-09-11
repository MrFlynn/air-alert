package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Displays information about this executable",
	Long:  "Displays the author, version, commit, and build date",
	Run:   printHelp,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func printHelp(cmd *cobra.Command, args []string) {
	var infoString string

	infoString += "   |\n"
	infoString += " .'|'.   Air Alert\n"
	infoString += fmt.Sprintf("/.'|\\ \\  Author: %s\n", viper.GetString("author"))
	infoString += fmt.Sprintf("| /|'.|  Version: %s\n", viper.GetString("version"))
	infoString += fmt.Sprintf(" \\ |\\/   Commit: %s\n", viper.GetString("commit"))
	infoString += fmt.Sprintf("  \\|/    Build Date: %s\n", viper.GetTime("date"))
	infoString += "   `\n"

	fmt.Print(infoString)
}
