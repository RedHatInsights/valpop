package cmd

import (
	"fmt"

	"github.com/RedHatInsights/valpop/impl/valkey"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Pop CMD
var dest string
var popCmd = &cobra.Command{
	Use:   "pop",
	Short: "copies to the dest for serving",
	Long:  "copies cache to dest for serving",
	RunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("dest") == "" {
			return fmt.Errorf("dest arg not set")
		}
		if viper.GetString("mode") == "valkey" {
			return valkey.PopFn(addr, dest)
		}
		return nil
	},
}

func init() {
	popCmd.Flags().StringVarP(&dest, "dest", "d", "", "Dest directory")
	viper.BindPFlag("dest", popCmd.Flags().Lookup("dest"))
	rootCmd.AddCommand(popCmd)
}

// TODO Define a cacher function to return all files in the standard struct
// TODO Stop passing prefix and other items to the cacher object
