package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/valkey-io/valkey-go"
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

		return populateFn(
			addr,
			viper.GetString("source"),
			viper.GetString("prefix"),
			viper.GetInt64("timeout"),
		)
	},
}

func init() {
	populateCmd.Flags().StringP("source", "s", "", "Source directory")
	populateCmd.Flags().StringP("prefix", "r", "", "Prefix for dir structure and cache")
	populateCmd.Flags().Int64P("timeout", "t", 10, "Timeout for cache")
	viper.BindPFlag("source", populateCmd.Flags().Lookup("source"))
	viper.BindPFlag("prefix", populateCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("timeout", populateCmd.Flags().Lookup("timeout"))
	rootCmd.AddCommand(populateCmd)
}

func populateFn(addr, source, prefix string, timeout int64) error {
	currentTime := time.Now()

	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	fileSystem := os.DirFS(source)
	cacher := NewCacher(ctx, client, currentTime, timeout)
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		return cacher.dumpFile(prefix, path, d, err)
	})
	if err != nil {
		fmt.Printf("%v", err)
	}

	cacher.cleanupCache(prefix)

	return nil
}
