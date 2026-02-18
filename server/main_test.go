package main

import (
	"sync"
	"testing"
)

func TestSetGet(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("key1", "value1")
	val, ok := kv.Get("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %s", val)
	}
}

func TestGetNonExistent(t *testing.T) {
	kv := NewMiniKV()

	_, ok := kv.Get("nonexistent")
	if ok {
		t.Error("Expected key not to exist")
	}
}

func TestDelete(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("key1", "value1")
	count := kv.Delete("key1")
	if count != 1 {
		t.Errorf("Expected 1, got %d", count)
	}

	_, ok := kv.Get("key1")
	if ok {
		t.Error("Expected key1 to be deleted")
	}
}

func TestDeleteMultiple(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("key1", "val1")
	kv.Set("key2", "val2")
	kv.Set("key3", "val3")

	count := kv.Delete("key1", "key2", "key3")
	if count != 3 {
		t.Errorf("Expected 3, got %d", count)
	}

	keys := kv.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys, got %d", len(keys))
	}
}

func TestKeys(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("a", "1")
	kv.Set("b", "2")
	kv.Set("c", "3")

	keys := kv.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
}

func TestFlushDB(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("key1", "value1")
	kv.Set("key2", "value2")

	kv.FlushDB()

	keys := kv.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after flush, got %d", len(keys))
	}
}

func TestConcurrentSetGet(t *testing.T) {
	kv := NewMiniKV()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kv.Set("key", "value")
			kv.Get("key")
		}(i)
	}

	wg.Wait()
}

func TestConcurrentMultipleKeys(t *testing.T) {
	kv := NewMiniKV()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kv.Set("key", "value")
			kv.Get("key")
			kv.Delete("key")
		}(i)
	}

	wg.Wait()
}

func TestConcurrentMixedOperations(t *testing.T) {
	kv := NewMiniKV()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kv.Set("key1", "value1")
			kv.Set("key2", "value2")
			kv.Get("key1")
			kv.Get("key2")
			kv.Keys()
			kv.Delete("key1")
		}(i)
	}

	wg.Wait()
}
