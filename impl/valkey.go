package impl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/valkey-io/valkey-go"
)

type Valkey struct {
	ctx    context.Context
	client valkey.Client
}

func NewValkey(addr string) (Valkey, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		panic(err)
	}
	return Valkey{
		ctx:    context.Background(),
		client: client,
	}, nil
}

func makeKey(namespace, filepath string, timestamp int64) string {
	return fmt.Sprintf("%s:%d:%s", namespace, timestamp, filepath)
}

func (v *Valkey) Close() {
	v.client.Close()
}

func (v *Valkey) SetItem(namespace, filepath string, timestamp int64, contents string) error {
	key := makeKey(namespace, filepath, timestamp)

	fmt.Printf("%s: %s (%d)\n", filepath, key, len(contents))

	err := v.client.Do(v.ctx, v.client.B().Set().Key(key).Value(string(contents)).Build()).Error()
	if err != nil {
		return fmt.Errorf("err from valkey:%w", err)
	}
	return nil
}

func (v *Valkey) GetKeys(namespace string) (AllItems, error) {
	cacheList := AllItems{namespace: Items{}}
	cursor := uint64(0)
	for {
		resp := v.client.Do(v.ctx, v.client.B().Scan().Cursor(cursor).Match(namespace+"*").Build())
		if resp.Error() != nil {
			return make(AllItems), fmt.Errorf("err from valkey:%w", resp.Error())
		}

		scan, err := resp.AsScanEntry()
		if err != nil {
			return make(AllItems), fmt.Errorf("scan decode error:%w", err)
		}

		for i := range scan.Elements {
			elems := strings.Split(scan.Elements[i], ":")
			timeStamp, err := strconv.Atoi(elems[1])
			if err != nil {
				return make(AllItems), err
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

func (v *Valkey) GetItem(namespace, filepath string, timestamp int64) (string, error) {
	key := makeKey(namespace, filepath, timestamp)
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

func (v *Valkey) DelKeys(allitems AllItems) error {
	keys := []string{}
	for namespace, items := range allitems {
		for filepath, timestamps := range items {
			for _, timestamp := range timestamps {
				keys = append(keys, makeKey(namespace, filepath, timestamp))
			}
		}
	}

	return v.client.Do(v.ctx, v.client.B().Del().Key(keys...).Build()).Error()
}
