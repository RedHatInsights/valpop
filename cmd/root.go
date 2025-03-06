package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var hostname string
var port string
var addr string

var rootCmd = &cobra.Command{
	Use:   "valpop",
	Short: "pops or populates Valkey for Frontends",
	Long:  "pops or populates Valkey for Frontends - ya know",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		addr = fmt.Sprintf("%s:%s", hostname, port)
	},
}

func init() {
	viper.SetEnvPrefix("VALPOP")
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().StringVarP(&hostname, "hostname", "a", "127.0.0.1", "Valkey hostname")
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", "6379", "Valkey port")
	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))

	rootCmd.AddCommand(popCmd)
	rootCmd.AddCommand(populateCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
