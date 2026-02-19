package internal

import "sync"

type MiniKV struct {
	store map[string]string
	mu    sync.RWMutex
}

func NewMiniKV() *MiniKV {
	return &MiniKV{store: make(map[string]string)}
}

func (kv *MiniKV) Set(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = value
	Debug("SET key=%q value=%q", key, value)
}

func (kv *MiniKV) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	val, ok := kv.store[key]
	if ok {
		Debug("GET key=%q value=%q", key, val)
	} else {
		Debug("GET key=%q not found", key)
	}
	return val, ok
}

func (kv *MiniKV) Delete(keys ...string) int {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	count := 0
	for _, key := range keys {
		if _, ok := kv.store[key]; ok {
			delete(kv.store, key)
			count++
			Debug("DEL key=%q", key)
		}
	}
	Debug("DEL deleted %d keys", count)
	return count
}

func (kv *MiniKV) Keys() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	keys := make([]string, 0, len(kv.store))
	for k := range kv.store {
		keys = append(keys, k)
	}
	Debug("KEYS returned %d keys", len(keys))
	return keys
}

func (kv *MiniKV) FlushDB() {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	count := len(kv.store)
	kv.store = make(map[string]string)
	Info("FLUSHDB deleted %d keys", count)
}

func (kv *MiniKV) GetAll() map[string]string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range kv.store {
		result[k] = v
	}
	return result
}

func (kv *MiniKV) SetFromMap(data map[string]string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	for k, v := range data {
		kv.store[k] = v
	}
}

func (kv *MiniKV) Len() int {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	return len(kv.store)
}
