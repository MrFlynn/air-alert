package cmd

import (
	"github.com/SherClockHolmes/webpush-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Generates VAPID keys for web push notifications",
	Long:  "Generates VAPID keys and stores them in the application config file",
	RunE:  generateKeys,
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
