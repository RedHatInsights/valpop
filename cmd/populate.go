package cmd

import (
	"fmt"

	"github.com/RedHatInsights/valpop/impl/s3"
	"github.com/RedHatInsights/valpop/impl/valkey"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Populate CMD
var populateCmd = &cobra.Command{
	Use:   "populate",
	Short: "populates the cache",
	Long:  "populates the cache from the source",
	RunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("source") == "" {
			return fmt.Errorf("no source arg set")
		}
		if viper.GetString("prefix") == "" {
			return fmt.Errorf("no prefix arg set")
		}

		if viper.GetString("mode") == "valkey" {
			client, err := valkey.NewValkey(addr)
			if err != nil {
				return err
			}

			defer client.Close()

			return client.PopulateFn(
				addr,
				viper.GetString("source"),
				viper.GetString("prefix"),
				viper.GetInt64("timeout"),
			)
		} else if viper.GetString("mode") == "s3" {
			client, err := s3.NewMinio(addr, viper.GetString("username"), viper.GetString("password"))
			if err != nil {
				return err
			}

			defer client.Close()
			return client.PopulateFn(
				addr,
				viper.GetString("source"),
				viper.GetString("prefix"),
				viper.GetInt64("timeout"),
			)
		}
		return nil
	},
}

func init() {
	populateCmd.Flags().StringP("source", "s", "", "Source directory")
	populateCmd.Flags().StringP("prefix", "r", "", "Prefix for dir structure and cache")
	populateCmd.Flags().Int64P("timeout", "t", 30, "Timeout for cache")
	viper.BindPFlag("source", populateCmd.Flags().Lookup("source"))
	viper.BindPFlag("prefix", populateCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("timeout", populateCmd.Flags().Lookup("timeout"))
	rootCmd.AddCommand(populateCmd)
}
