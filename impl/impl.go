package impl

type AllItems map[string]Items

type Items map[string][]int64

type CacheInterface interface {
	SetItem(namespace, filename string, timestamp int64, contents string) error
	GetKeys(namespace string) (AllItems, error)
	GetItem(namespace, filename string, timestamp int64) (string, error)
	DelKeys(AllItems) error
	Close()
}
