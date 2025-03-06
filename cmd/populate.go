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
var source string
var prefix string
var timeout int64
var populateCmd = &cobra.Command{
	Use:   "populate",
	Short: "populates the cache",
	Long:  "populates the cache from the source",
	Run: func(cmd *cobra.Command, args []string) {
		populateFn(addr, source, prefix, timeout)
	},
}

func init() {
	populateCmd.Flags().StringVarP(&source, "source", "s", "", "Source directory")
	populateCmd.MarkFlagRequired("source")
	populateCmd.Flags().StringVarP(&prefix, "prefix", "r", "", "Prefix for dir structure and cache")
	populateCmd.MarkFlagRequired("prefix")
	populateCmd.Flags().Int64VarP(&timeout, "timeout", "t", 10, "Timeout for cache")
	populateCmd.MarkFlagRequired("source")
	populateCmd.MarkFlagRequired("prefix")
	viper.BindPFlag("source", populateCmd.Flags().Lookup("source"))
	viper.BindPFlag("prefix", populateCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("timeout", populateCmd.Flags().Lookup("timeout"))
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
	cacher := NewCacher(ctx, client, prefix, currentTime, timeout)
	err = fs.WalkDir(fileSystem, ".", cacher.dumpFile)
	if err != nil {
		fmt.Printf("%v", err)
	}

	cacher.cleanupCache()

	return nil
}
