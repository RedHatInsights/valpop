package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/RedHatInsights/valpop/impl"
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

	client, err := impl.NewValkey(addr)
	if err != nil {
		return err
	}

	defer client.Close()

	fileSystem := os.DirFS(source)
	err = fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		return dumpFile(&client, prefix, path, d, fmt.Sprintf("%d", currentTime.Unix()), err)
	})
	if err != nil {
		fmt.Printf("%v", err)
	}

	cleanupCache(&client, prefix, timeout)

	return nil
}

func dumpFile(client impl.CacheInterface, prefix, path string, d fs.DirEntry, timestamp string, err error) error {
	if err != nil {
		fmt.Printf("WE GOT AN ERR %v", err)
	}

	if d.IsDir() {
		return nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s:%s:%s", prefix, timestamp, path)

	fmt.Printf("%s: %s (%d)\n", path, key, len(contents))

	timestampAsInt, err := strconv.Atoi(timestamp)
	if err != nil {
		return err
	}
	err = client.SetItem(prefix, path, int64(timestampAsInt), string(contents))
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}

	return nil
}

func cleanupCache(client impl.CacheInterface, prefix string, timeout int64) error {
	// Soemthign like  filename[4,5,6,7]
	cacheList, err := client.GetKeys(prefix)
	if err != nil {
		return err
	}
	deleteItems := make(impl.AllItems)
	deleteItems[prefix] = make(impl.Items)
	for filename, stamps := range cacheList[prefix] {

		slices.Sort(stamps)
		for z := range stamps[1:] {
			if stamps[z] < time.Now().Unix()-timeout {
				fmt.Printf("del: %s:%d\n", filename, stamps[z])
				deleteItems[prefix][filename] = append(deleteItems[prefix][filename], stamps[z])
			}
		}
	}
	fmt.Printf("%v", deleteItems)
	err = client.DelKeys(deleteItems)
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	return nil
}
