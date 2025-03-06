package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/valkey-io/valkey-go"
)

type Cacher struct {
	client    valkey.Client
	ctx       context.Context
	prefix    string
	cacheTime time.Time
	timeout   int64
}

func (c *Cacher) dumpFile(path string, d fs.DirEntry, err error) error {
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

	key := fmt.Sprintf("%s:%d:%s", c.prefix, c.cacheTime.Unix(), path)

	fmt.Printf("%s: %s (%d)\n", path, key, len(contents))

	err = c.client.Do(c.ctx, c.client.B().Set().Key(key).Value(string(contents)).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}

	return nil
}

func (c *Cacher) cleanupCache() error {
	// Soemthign like  filename[4,5,6,7]
	cacheList := make(map[string][]int64)
	cursor := uint64(0)
	for {
		resp := c.client.Do(c.ctx, c.client.B().Scan().Cursor(cursor).Match(c.prefix+"*").Build())
		if resp.Error() != nil {
			return fmt.Errorf("err from valkey:%w", resp.Error())
		}

		scan, err := resp.AsScanEntry()
		if err != nil {
			return fmt.Errorf("scan decode error:%w", err)
		}

		for i := range scan.Elements {
			elems := strings.Split(scan.Elements[i], ":")
			timeStamp, err := strconv.Atoi(elems[1])
			if err != nil {
				return err
			}
			cacheList[elems[2]] = append(cacheList[elems[2]], int64(timeStamp))
		}

		if scan.Cursor == 0 {
			break
		}
		cursor = scan.Cursor
	}
	c.processCacheList(cacheList)
	return nil
}

func (c *Cacher) processCacheList(cacheList map[string][]int64) error {
	keys := []string{}
	for filename, stamps := range cacheList {
		sort.Slice(stamps, func(i, j int) bool {
			return stamps[i] < stamps[j] // Ascending order
		})
		for z := range stamps[1:] {
			if stamps[z] < time.Now().Unix()-c.timeout {
				fmt.Printf("del: %s:%d\n", filename, stamps[z])
				keys = append(keys, fmt.Sprintf("%s:%d:%s", c.prefix, stamps[z], filename))
			}
		}
	}
	fmt.Printf("%v", keys)
	err := c.client.Do(c.ctx, c.client.B().Del().Key(keys...).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	return nil
}

func NewCacher(ctx context.Context, client valkey.Client, prefix string, cacheTime time.Time, timeout int64) Cacher {
	return Cacher{
		client:    client,
		ctx:       ctx,
		prefix:    prefix,
		cacheTime: cacheTime,
		timeout:   timeout,
	}
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

func writeFile(root, key, contents string) {
	elems := strings.Split(key, ":")
	path := filepath.Join(root, elems[2])
	dir, filename := filepath.Split(path)
	fmt.Printf("%s - %s\n", dir, filename)
	os.MkdirAll(dir, os.ModePerm)
	os.WriteFile(path, []byte(contents), 0664)
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

// docker run --replace --name some-valkey --network host -d valkey/valkey
// docker run -it --network host --rm valkey/valkey valkey-cli -h 127.0.0.1

func main() {
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

	viper.SetEnvPrefix("VALPOP")
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().StringVarP(&hostname, "hostname", "a", "127.0.0.1", "Valkey hostname")
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", "6379", "Valkey port")
	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))

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

	// Pop CMD
	var dest string
	var popCmd = &cobra.Command{
		Use:   "pop",
		Short: "copies to the dest for serving",
		Long:  "copies cache to dest for serving",
		Run: func(cmd *cobra.Command, args []string) {
			popFn(addr, dest)
		},
	}

	popCmd.Flags().StringVarP(&dest, "dest", "d", "", "Dest directory")
	popCmd.MarkFlagRequired("dest")
	viper.BindPFlag("dest", popCmd.Flags().Lookup("dest"))

	rootCmd.AddCommand(popCmd)
	rootCmd.AddCommand(populateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("%s", err)
		os.Exit(127)
	}
}
