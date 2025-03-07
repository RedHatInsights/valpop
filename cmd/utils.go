package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

type Cacher struct {
	client    valkey.Client
	ctx       context.Context
	cacheTime time.Time
	timeout   int64
}

func (c *Cacher) dumpFile(prefix, path string, d fs.DirEntry, err error) error {
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

	key := fmt.Sprintf("%s:%d:%s", prefix, c.cacheTime.Unix(), path)

	fmt.Printf("%s: %s (%d)\n", path, key, len(contents))

	err = c.client.Do(c.ctx, c.client.B().Set().Key(key).Value(string(contents)).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}

	return nil
}

func (c *Cacher) cleanupCache(prefix string) error {
	// Soemthign like  filename[4,5,6,7]
	cacheList := make(map[string][]int64)
	cursor := uint64(0)
	for {
		resp := c.client.Do(c.ctx, c.client.B().Scan().Cursor(cursor).Match(prefix+"*").Build())
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
	c.processCacheList(cacheList, prefix)
	return nil
}

func (c *Cacher) processCacheList(cacheList map[string][]int64, prefix string) error {
	keys := []string{}
	for filename, stamps := range cacheList {
		sort.Slice(stamps, func(i, j int) bool {
			return stamps[i] < stamps[j] // Ascending order
		})
		for z := range stamps[1:] {
			if stamps[z] < time.Now().Unix()-c.timeout {
				fmt.Printf("del: %s:%d\n", filename, stamps[z])
				keys = append(keys, fmt.Sprintf("%s:%d:%s", prefix, stamps[z], filename))
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

func NewCacher(ctx context.Context, client valkey.Client, cacheTime time.Time, timeout int64) Cacher {
	return Cacher{
		client:    client,
		ctx:       ctx,
		cacheTime: cacheTime,
		timeout:   timeout,
	}
}
