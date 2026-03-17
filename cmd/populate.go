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
		if viper.GetString("image") == "" {
			return fmt.Errorf("no image arg set")
		}

		minAssetRecords := viper.GetInt("min-asset-records")
		if minAssetRecords < 0 {
			return fmt.Errorf("min-asset-records must be a non-negative integer")
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
				viper.GetString("image"),
				viper.GetInt64("timeout"),
				int64(minAssetRecords),
				viper.GetInt64("cache-max-age"),
			)
		} else if viper.GetString("mode") == "s3" {
			client, err := s3.NewMinio(addr, viper.GetString("username"), viper.GetString("password"))
			if err != nil {
				return err
			}

			defer client.Close()
			return client.PopulateFn(
				addr,
				bucket,
				viper.GetString("source"),
				viper.GetString("prefix"),
				viper.GetString("image"),
				viper.GetInt64("timeout"),
				int64(minAssetRecords),
				viper.GetInt64("cache-max-age"),
			)
		}
		return nil
	},
}

func init() {
	populateCmd.Flags().StringP("source", "s", "", "Source directory")
	populateCmd.Flags().StringP("prefix", "r", "", "Prefix for dir structure and cache")
	populateCmd.Flags().StringP("image", "i", "", "Image identifier (e.g., container image tag)")
	populateCmd.Flags().Int64P("timeout", "t", 30, "Timeout for cache")
	populateCmd.Flags().IntP("min-asset-records", "n", 3, "Minimum number of asset records to keep")
	populateCmd.Flags().Int64P("cache-max-age", "g", 86400, "Cache-Control max-age in seconds for static assets")
	viper.BindPFlag("source", populateCmd.Flags().Lookup("source"))
	viper.BindPFlag("prefix", populateCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("image", populateCmd.Flags().Lookup("image"))
	viper.BindPFlag("timeout", populateCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("min-asset-records", populateCmd.Flags().Lookup("min-asset-records"))
	viper.BindPFlag("cache-max-age", populateCmd.Flags().Lookup("cache-max-age"))
	rootCmd.AddCommand(populateCmd)
}
