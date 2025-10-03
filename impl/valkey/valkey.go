package valkey

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	fp "path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	impl "github.com/RedHatInsights/valpop/impl"
	vkc "github.com/valkey-io/valkey-go"
)

type Valkey struct {
	ctx    context.Context
	client vkc.Client
}

func NewValkey(addr string) (Valkey, error) {
	client, err := vkc.NewClient(vkc.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		panic(err)
	}
	return Valkey{
		ctx:    context.Background(),
		client: client,
	}, nil
}

func makeDataKey(namespace, filepath string, timestamp int64) string {
	return fmt.Sprintf("data:%s:%d:%s", namespace, timestamp, filepath)
}

func makeLockKey(namespace string, timestamp int64) string {
	return fmt.Sprintf("lock:%s:%d", namespace, timestamp)
}

func (v *Valkey) Close() {
	v.client.Close()
}

func (v *Valkey) StartPopulate(namespace string, timestamp int64) error {
	lockKey := makeLockKey(namespace, timestamp)
	err := v.client.Do(v.ctx, v.client.B().Set().Key(lockKey).Value("in-progress").Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	fmt.Printf("%s:%d (in-progress)\n", namespace, timestamp)
	return nil
}

func (v *Valkey) EndPopulate(namespace string, timestamp int64) error {
	lockKey := makeLockKey(namespace, timestamp)
	err := v.client.Do(v.ctx, v.client.B().Del().Key(lockKey).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	fmt.Printf("%s:%d (removed)\n", namespace, timestamp)
	return nil
}

func (v *Valkey) SetItem(namespace, filepath string, timestamp int64, contents string) error {
	key := makeDataKey(namespace, filepath, timestamp)

	fmt.Printf("%s: %s (%d)\n", filepath, key, len(contents))

	err := v.client.Do(v.ctx, v.client.B().Set().Key(key).Value(string(contents)).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	return nil
}

func (v *Valkey) GetKeys(namespace string) (impl.AllItems, error) {
	cacheList := impl.AllItems{namespace: impl.Items{}}
	cursor := uint64(0)
	for {
		resp := v.client.Do(v.ctx, v.client.B().Scan().Cursor(cursor).Match("data:"+namespace+":*").Build())
		if resp.Error() != nil {
			return make(impl.AllItems), fmt.Errorf("err from valkey:%w", resp.Error())
		}

		scan, err := resp.AsScanEntry()
		if err != nil {
			return make(impl.AllItems), fmt.Errorf("scan decode error:%w", err)
		}

		for i := range scan.Elements {
			elems := strings.Split(scan.Elements[i], ":")[1:]
			timeStamp, err := strconv.Atoi(elems[1])
			if err != nil {
				return make(impl.AllItems), err
			}

			lockKey := makeLockKey(namespace, int64(timeStamp))
			inProgress, err := v.isInProgress(lockKey)
			if inProgress {
				continue
			}
			if err != nil {
				return make(impl.AllItems), err
			}

			cacheList[namespace][elems[2]] = append(cacheList[namespace][elems[2]], int64(timeStamp))
		}

		if scan.Cursor == 0 {
			break
		}
		cursor = scan.Cursor
	}
	return cacheList, nil
}

func (v *Valkey) isInProgress(lockKey string) (bool, error) {
	resp := v.client.Do(v.ctx, v.client.B().Get().Key(lockKey).Build())
	if resp.Error() != nil {
		return false, nil
	}
	data, err := resp.ToString()
	if err != nil {
		return false, err
	}
	return data == "in-progress", nil
}

func (v *Valkey) GetItem(namespace, filepath string, timestamp int64) (string, error) {
	key := makeDataKey(namespace, filepath, timestamp)
	resp := v.client.Do(v.ctx, v.client.B().Get().Key(key).Build())
	if resp.Error() != nil {
		return "", nil
	}
	contents, err := resp.ToString()
	if err != nil {
		return "", nil
	}
	return contents, nil
}

func (v *Valkey) DelKeys(allitems impl.AllItems) error {
	fmt.Printf("Deleting %d keys", len(impl.AllItems{}))
	keys := []string{}
	for namespace, items := range allitems {
		for filepath, timestamps := range items {
			for _, timestamp := range timestamps {
				keys = append(keys, makeDataKey(namespace, filepath, timestamp))
			}
		}
	}

	return v.client.Do(v.ctx, v.client.B().Del().Key(keys...).Build()).Error()
}

func PopFn(addr, dest string) error {
	fmt.Println("Invoking pop...")
	client, err := NewValkey(addr)
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

func (v *Valkey) PopulateFn(addr, source, prefix string, timeout int64, minAssetRecords int64) error {
	currentTime := time.Now().Unix()

	fileSystem := os.DirFS(source)
	v.StartPopulate(prefix, currentTime)
	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		return dumpFile(v, fileSystem, prefix, path, d, fmt.Sprintf("%d", currentTime), err)
	})
	if err != nil {
		fmt.Printf("%v", err)
	}
	v.EndPopulate(prefix, currentTime)
	cleanupCache(v, prefix, timeout, minAssetRecords)
	return nil
}

func dumpFile(client *Valkey, fileSystem fs.FS, prefix, path string, d fs.DirEntry, timestamp string, err error) error {
	if err != nil {
		fmt.Printf("WE GOT AN ERR %v", err)
	}

	if d.IsDir() {
		return nil
	}

	contents, err := fs.ReadFile(fileSystem, path)
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

func cleanupCache(client *Valkey, prefix string, timeout int64, minAssetRecords int64) error {
	// Get all cached items for this prefix
	cacheList, err := client.GetKeys(prefix)
	if err != nil {
		return err
	}
	deleteItems := make(impl.AllItems)
	deleteItems[prefix] = make(impl.Items)

	for filename, stamps := range cacheList[prefix] {
		// Sort stamps in descending order (newest first)
		slices.Sort(stamps)
		slices.Reverse(stamps)

		// Determine how many versions to keep for this file
		keepCount := minAssetRecords
		if keepCount > int64(len(stamps)) {
			keepCount = int64(len(stamps))
		}

		// Check versions beyond the minimum required
		for i := int(keepCount); i < len(stamps); i++ {
			timestamp := stamps[i]
			// Only delete if it's also older than the timeout
			if time.Now().Unix()-timestamp > timeout {
				fmt.Printf("del: %s:%d\n", filename, timestamp)
				deleteItems[prefix][filename] = append(deleteItems[prefix][filename], timestamp)
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
