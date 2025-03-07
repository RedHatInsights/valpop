package cmd

import (
	"fmt"
	"os"
	fp "path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RedHatInsights/valpop/impl"
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
		return popFn(addr, dest)
	},
}

func init() {
	popCmd.Flags().StringVarP(&dest, "dest", "d", "", "Dest directory")
	viper.BindPFlag("dest", popCmd.Flags().Lookup("dest"))
	rootCmd.AddCommand(popCmd)
}

func popFn(addr, dest string) error {
	fmt.Println("Invoking pop...")
	client, err := impl.NewValkey(addr)
	if err != nil {
		return err
	}

	defer client.Close()

	allKeys, err := client.GetKeys("")
	if err != nil {
		return err
	}

	for prefix, fileitems := range allKeys {
		for filepath, stamps := range fileitems {
			slices.Sort(stamps)
			contents, err := client.GetItem(prefix, filepath, stamps[0])
			if err != nil {
				return err
			}
			writeFile(dest, filepath, contents)
		}
	}
	return nil
}

func writeFile(root, filepath, contents string) {
	path := fp.Join(root, filepath)
	dir, filename := fp.Split(path)
	fmt.Printf("%s - %s\n", dir, filename)
	os.MkdirAll(dir, os.ModePerm)
	os.WriteFile(path, []byte(contents), 0664)
}

// TODO Define a cacher function to return all files in the standard struct
// TODO Stop passing prefix and other items to the cacher object
