package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/valkey-io/valkey-go"
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
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	cursor := uint64(0)

	for {
		resp := client.Do(ctx, client.B().Scan().Cursor(cursor).Build())
		if resp.Error() != nil {
			return fmt.Errorf("err from valkey:%w", resp.Error())
		}

		scan, err := resp.AsScanEntry()
		if err != nil {
			return fmt.Errorf("scan decode error:%w", err)
		}

		for i := range scan.Elements {
			resp := client.Do(ctx, client.B().Get().Key(scan.Elements[i]).Build())
			contents, err := resp.ToString()
			if err != nil {
				return err
			}
			writeFile(dest, scan.Elements[i], contents)
		}
		fmt.Printf("%d\n", scan.Cursor)

		if scan.Cursor == 0 {
			break
		}
		cursor = scan.Cursor
	}
	return nil
}

func writeFile(root, key, contents string) {
	elems := strings.Split(key, ":")
	path := filepath.Join(root, elems[2])
	dir, filename := filepath.Split(path)
	fmt.Printf("%s - %s\n", dir, filename)
	os.MkdirAll(dir, os.ModePerm)
	os.WriteFile(path, []byte(contents), 0664)
}

// TODO Define a cacher function to return all files in the standard struct
// TODO Stop passing prefix and other items to the cacher object
