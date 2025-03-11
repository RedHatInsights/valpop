package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var addr string

var rootCmd = &cobra.Command{
	Use:   "valpop",
	Short: "pops or populates Valkey for Frontends",
	Long:  "pops or populates Valkey for Frontends - ya know",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		addr = fmt.Sprintf("%s:%s", viper.GetString("hostname"), viper.GetString("port"))
		if viper.GetString("mode") == "s3" {
			if viper.GetString("username") == "" {
				return fmt.Errorf("can't have s3 with no username")
			}
			if viper.GetString("password") == "" {
				return fmt.Errorf("can't have s3 with no password")
			}
		}
		return nil
	},
}

func init() {
	viper.SetEnvPrefix("VALPOP")
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().StringP("hostname", "a", "127.0.0.1", "Valkey hostname")
	rootCmd.PersistentFlags().StringP("port", "p", "6379", "Valkey port")
	rootCmd.PersistentFlags().StringP("mode", "m", "s3", "Mode, s3 or valkey")
	rootCmd.PersistentFlags().StringP("username", "u", "", "Username for S3")
	rootCmd.PersistentFlags().StringP("password", "c", "", "Password for S3")
	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("mode", rootCmd.PersistentFlags().Lookup("mode"))
	viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
}

func Execute() error {
	return rootCmd.Execute()
}
