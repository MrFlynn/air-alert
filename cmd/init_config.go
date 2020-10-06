package cmd

import (
	"github.com/SherClockHolmes/webpush-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var keysCmd = &cobra.Command{
	Use:   "init-config",
	Short: "Initializes configuration file with defaults.",
	Long: `Initializes existing, empty configuration file with program defaults as well as generating
VAPID keys and stores them in the configuration file`,
	RunE: generateKeys,
}

func init() {
	rootCmd.AddCommand(keysCmd)
}

func generateKeys(cmd *cobra.Command, args []string) error {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return err
	}

	viper.Set("web.notifications.private_key", privateKey)
	viper.Set("web.notifications.public_key", publicKey)

	return viper.WriteConfig()
}
