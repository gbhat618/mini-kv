package main

import (
	"net"
	"strings"
	"sync"
	"sync/atomic"
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

func TestTransactionBeginCommit(t *testing.T) {
	s := NewServer(":6379")
	kv := s.kv

	kv.Set("key1", "original")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	resp := s.processCommand("BEGIN", txn, &inTxn)
	if resp != "+OK\r\n" {
		t.Errorf("Expected OK, got %s", resp)
	}
	if !inTxn {
		t.Error("Expected inTxn to be true")
	}

	resp = s.processCommand("SET key1 modified", txn, &inTxn)
	if resp != "+OK\r\n" {
		t.Errorf("Expected OK, got %s", resp)
	}

	resp = s.processCommand("GET key1", txn, &inTxn)
	if !contains(resp, "modified") {
		t.Errorf("Expected modified, got %s", resp)
	}

	val, _ := kv.Get("key1")
	if val != "original" {
		t.Errorf("Expected original (not visible outside txn), got %s", val)
	}

	resp = s.processCommand("COMMIT", txn, &inTxn)
	if resp != "+OK\r\n" {
		t.Errorf("Expected OK, got %s", resp)
	}

	val, _ = kv.Get("key1")
	if val != "modified" {
		t.Errorf("Expected modified after commit, got %s", val)
	}
}

func TestTransactionRollback(t *testing.T) {
	s := NewServer(":6379")
	kv := s.kv

	kv.Set("key1", "original")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	s.processCommand("BEGIN", txn, &inTxn)
	s.processCommand("SET key1 modified", txn, &inTxn)

	resp := s.processCommand("ROLLBACK", txn, &inTxn)
	if resp != "+OK\r\n" {
		t.Errorf("Expected OK, got %s", resp)
	}

	val, _ := kv.Get("key1")
	if val != "original" {
		t.Errorf("Expected original after rollback, got %s", val)
	}
}

func TestTransactionMultipleKeys(t *testing.T) {
	s := NewServer(":6379")
	kv := s.kv

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	s.processCommand("BEGIN", txn, &inTxn)
	s.processCommand("SET key1 val1", txn, &inTxn)
	s.processCommand("SET key2 val2", txn, &inTxn)
	s.processCommand("SET key3 val3", txn, &inTxn)
	s.processCommand("DEL key2", txn, &inTxn)

	_ = s.processCommand("COMMIT", txn, &inTxn)

	keys := kv.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	_, ok := kv.Get("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	_, ok = kv.Get("key2")
	if ok {
		t.Error("Expected key2 to be deleted")
	}
	_, ok = kv.Get("key3")
	if !ok {
		t.Error("Expected key3 to exist")
	}
}

func TestTransactionIsolation(t *testing.T) {
	s := NewServer(":6379")
	kv := s.kv

	kv.Set("key1", "original")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	s.processCommand("BEGIN", txn, &inTxn)
	s.processCommand("SET key1 modified", txn, &inTxn)

	resp := s.processCommand("GET key1", txn, &inTxn)
	if !contains(resp, "modified") {
		t.Errorf("Expected modified in transaction, got %s", resp)
	}

	val, _ := kv.Get("key1")
	if val != "original" {
		t.Errorf("Expected original in main store (isolation), got %s", val)
	}

	s.processCommand("COMMIT", txn, &inTxn)

	val, _ = kv.Get("key1")
	if val != "modified" {
		t.Errorf("Expected modified after commit, got %s", val)
	}
}

func TestTransactionNested(t *testing.T) {
	s := NewServer(":6379")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := true

	resp := s.processCommand("BEGIN", txn, &inTxn)
	if !contains(resp, "transaction already in progress") {
		t.Errorf("Expected error for nested BEGIN, got %s", resp)
	}
}

func TestTransactionNoCommit(t *testing.T) {
	s := NewServer(":6379")
	kv := s.kv

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	s.processCommand("BEGIN", txn, &inTxn)
	s.processCommand("SET key1 value1", txn, &inTxn)

	_ = s.processCommand("COMMIT", txn, &inTxn)
	s.processCommand("GET key1", txn, &inTxn)

	val, _ := kv.Get("key1")
	if val != "value1" {
		t.Errorf("Expected value1 after commit, got %s", val)
	}
}

func TestTransactionRollbackWithoutBegin(t *testing.T) {
	s := NewServer(":6379")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	resp := s.processCommand("COMMIT", txn, &inTxn)
	if !contains(resp, "no transaction in progress") {
		t.Errorf("Expected error for COMMIT without BEGIN, got %s", resp)
	}

	resp = s.processCommand("ROLLBACK", txn, &inTxn)
	if !contains(resp, "no transaction in progress") {
		t.Errorf("Expected error for ROLLBACK without BEGIN, got %s", resp)
	}
}

func TestTransactionFlushDBBlocked(t *testing.T) {
	s := NewServer(":6379")

	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	s.processCommand("BEGIN", txn, &inTxn)

	resp := s.processCommand("FLUSHDB", txn, &inTxn)
	if !contains(resp, "cannot FLUSHDB within a transaction") {
		t.Errorf("Expected error for FLUSHDB in txn, got %s", resp)
	}
}

func TestMaxValueSizeValidation(t *testing.T) {
	s := NewServer(":6379")

	origMaxValue := maxValueSize
	maxValueSize = 10
	defer func() { maxValueSize = origMaxValue }()

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("SET key verylongvalue", txn, &inTxn)
	if !strings.Contains(resp, "value size exceeds maximum") {
		t.Errorf("Expected value size error, got: %s", resp)
	}

	resp = s.processCommand("SET key short", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("Expected OK for valid value, got: %s", resp)
	}
}

func TestConnectionLimit(t *testing.T) {
	origMaxConns := maxConns
	maxConns = 2
	defer func() { maxConns = origMaxConns }()

	origAuth := authPassword
	authPassword = ""
	defer func() { authPassword = origAuth }()

	server := NewServer(":0")

	var connCount int32

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", server.addr)
			if err == nil {
				atomic.AddInt32(&connCount, 1)
				conn.Close()
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&connCount) > int32(maxConns) {
		t.Errorf("Expected connection count to be limited to %d, got %d", maxConns, connCount)
	}
}

func TestAllCommands(t *testing.T) {
	s := NewServer(":6379")
	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("SET key1 value1", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("SET failed: %s", resp)
	}

	resp = s.processCommand("GET key1", txn, &inTxn)
	if !strings.Contains(resp, "value1") {
		t.Errorf("GET failed: %s", resp)
	}

	resp = s.processCommand("KEYS", txn, &inTxn)
	if !strings.Contains(resp, "key1") {
		t.Errorf("KEYS failed: %s", resp)
	}

	resp = s.processCommand("DEL key1", txn, &inTxn)
	if !strings.Contains(resp, "1") {
		t.Errorf("DEL failed: %s", resp)
	}

	resp = s.processCommand("PING", txn, &inTxn)
	if !strings.Contains(resp, "PONG") {
		t.Errorf("PING failed: %s", resp)
	}
}

func TestTransactionWithCommands(t *testing.T) {
	s := NewServer(":6379")
	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	s.processCommand("SET initial value", txn, &inTxn)

	resp := s.processCommand("BEGIN", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("BEGIN failed: %s", resp)
	}

	resp = s.processCommand("SET key1 val1", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("SET in txn failed: %s", resp)
	}

	resp = s.processCommand("GET key1", txn, &inTxn)
	if !strings.Contains(resp, "val1") {
		t.Errorf("GET in txn failed: %s", resp)
	}

	resp = s.processCommand("COMMIT", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("COMMIT failed: %s", resp)
	}

	resp = s.processCommand("GET key1", txn, &inTxn)
	if !strings.Contains(resp, "val1") {
		t.Errorf("GET after commit failed: %s", resp)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
