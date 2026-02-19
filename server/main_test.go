package main

import (
	"encoding/json"
	"fmt"
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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")

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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")

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
	s := NewServer(":6379", ":16379")

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
	s := NewServer(":6379", ":16379")

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

	server := NewServer(":0", ":0")

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
	s := NewServer(":6379", ":16379")
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
	s := NewServer(":6379", ":16379")
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

func TestConsistentHash(t *testing.T) {
	ch := NewConsistentHash(150)

	ch.AddNode("node1", "localhost:6379")
	ch.AddNode("node2", "localhost:6380")
	ch.AddNode("node3", "localhost:6381")

	keys := []string{}
	for i := 0; i < 100; i++ {
		keys = append(keys, fmt.Sprintf("key%d", i))
	}
	for i := 0; i < 100; i++ {
		keys = append(keys, fmt.Sprintf("user:%d", i))
	}
	for i := 0; i < 100; i++ {
		keys = append(keys, fmt.Sprintf("session:%d", i))
	}

	nodeCounts := make(map[string]int)
	for _, key := range keys {
		node := ch.GetNode(key)
		if node == "" {
			t.Errorf("Expected node for key %s, got empty", key)
		}
		nodeCounts[node]++
	}

	if len(nodeCounts) != 3 {
		t.Errorf("Expected keys distributed across 3 nodes, got %d", len(nodeCounts))
	}

	for node, count := range nodeCounts {
		t.Logf("Node %s has %d keys", node, count)
	}

	ch.RemoveNode("node2", "localhost:6380")

	allNodes := ch.GetAllNodes()
	if len(allNodes) != 2 {
		t.Errorf("Expected 2 nodes after removal, got %d", len(allNodes))
	}
}

func TestClusterInfo(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	if s.cluster == nil {
		t.Error("Expected cluster to be initialized")
	}

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("CLUSTER INFO", txn, &inTxn)
	if !strings.Contains(resp, "cluster_enabled: true") {
		t.Errorf("Expected cluster enabled, got: %s", resp)
	}
}

func TestClusterMembers(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	s.cluster.AddNode("node1", "localhost:6379", RoleMaster)
	s.cluster.AddNode("node2", "localhost:6380", RoleMaster)
	s.cluster.AddNode("node3", "localhost:6381", RoleMaster)

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("CLUSTER MEMBERS", txn, &inTxn)
	if !strings.Contains(resp, "node1") || !strings.Contains(resp, "node2") || !strings.Contains(resp, "node3") {
		t.Errorf("Expected all nodes in MEMBERS response, got: %s", resp)
	}
}

func TestClusterKeyDistribution(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	s.cluster.AddNode("node1", "localhost:6379", RoleMaster)
	s.cluster.AddNode("node2", "localhost:6380", RoleMaster)
	s.cluster.AddNode("node3", "localhost:6381", RoleMaster)

	testKeys := []string{}
	for i := 0; i < 50; i++ {
		testKeys = append(testKeys, fmt.Sprintf("user:%d", i))
		testKeys = append(testKeys, fmt.Sprintf("session:%d", i))
		testKeys = append(testKeys, fmt.Sprintf("cache:key%d", i))
	}

	distribution := make(map[string]int)
	for _, key := range testKeys {
		node := s.cluster.GetNodeForKey(key)
		distribution[node]++
	}

	if len(distribution) < 2 {
		t.Errorf("Expected keys distributed across at least 2 nodes, got %d nodes", len(distribution))
	}

	for node, count := range distribution {
		t.Logf("Node %s has %d keys", node, count)
	}
}

func TestClusterInfoWithoutClusterMode(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = false
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("CLUSTER INFO", txn, &inTxn)
	if !strings.Contains(resp, "cluster mode not enabled") {
		t.Errorf("Expected cluster not enabled error, got: %s", resp)
	}
}

func TestServerInfo(t *testing.T) {
	s := NewServer(":6379", ":16379")
	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("INFO", txn, &inTxn)
	if !strings.Contains(resp, "mini-kv server") {
		t.Errorf("Expected server info, got: %s", resp)
	}
}

func TestReplicationState(t *testing.T) {
	rs := NewReplicationState()

	if rs.GetRole() != RoleMaster {
		t.Errorf("Expected default role to be master, got %s", rs.GetRole())
	}

	rs.SetMaster("localhost:6379")
	if rs.GetRole() != RoleSlave {
		t.Errorf("Expected role to be slave, got %s", rs.GetRole())
	}
	if rs.GetMasterAddr() != "localhost:6379" {
		t.Errorf("Expected master addr to be localhost:6379, got %s", rs.GetMasterAddr())
	}

	rs.SetMaster("")
	if rs.GetRole() != RoleMaster {
		t.Errorf("Expected role to be master after SetMaster(\"\"), got %s", rs.GetRole())
	}
}

func TestReplicationStateAddSlave(t *testing.T) {
	rs := NewReplicationState()

	rs.AddSlave("localhost:6380")
	rs.AddSlave("localhost:6381")

	slaves := rs.GetSlaveAddrs()
	if len(slaves) != 2 {
		t.Errorf("Expected 2 slaves, got %d", len(slaves))
	}

	rs.RemoveSlave("localhost:6380")
	slaves = rs.GetSlaveAddrs()
	if len(slaves) != 1 {
		t.Errorf("Expected 1 slave after removal, got %d", len(slaves))
	}
}

func TestRoleCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("ROLE", txn, &inTxn)
	if !strings.Contains(resp, "role: master") {
		t.Errorf("Expected role master, got: %s", resp)
	}
}

func TestReplicationInfoCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	s.cluster.replication.AddSlave("localhost:6380")
	s.cluster.replication.AddSlave("localhost:6381")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("INFO REPLICATION", txn, &inTxn)
	if !strings.Contains(resp, "role: master") {
		t.Errorf("Expected role master in replication info, got: %s", resp)
	}
	if !strings.Contains(resp, "connected_slaves: 2") {
		t.Errorf("Expected 2 connected slaves, got: %s", resp)
	}
}

func TestReplicaOfCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("REPLICAOF localhost 6379", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("Expected OK for replicaof, got: %s", resp)
	}

	if s.cluster.replication.GetRole() != RoleSlave {
		t.Errorf("Expected role to be slave, got %s", s.cluster.replication.GetRole())
	}
	if s.cluster.replication.GetMasterAddr() != "localhost:6379" {
		t.Errorf("Expected master addr, got %s", s.cluster.replication.GetMasterAddr())
	}

	resp = s.processCommand("REPLICAOF NO ONE", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("Expected OK for replicaof no one, got: %s", resp)
	}

	if s.cluster.replication.GetRole() != RoleMaster {
		t.Errorf("Expected role to be master after NO ONE, got %s", s.cluster.replication.GetRole())
	}
}

func TestSyncCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	s.kv.Set("key1", "value1")
	s.kv.Set("key2", "value2")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("SYNC", txn, &inTxn)
	if !strings.Contains(resp, "key1") || !strings.Contains(resp, "key2") {
		t.Errorf("Expected sync data, got: %s", resp)
	}
}

func TestReplicaWriteCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	s.cluster.replication.SetMaster("localhost:6379")

	data := map[string]string{"key1": "value1", "key2": "value2"}
	dataJSON, _ := json.Marshal(data)

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand(fmt.Sprintf("REPLICA_WRITE %s", dataJSON), txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("Expected OK for replica write, got: %s", resp)
	}

	val, ok := s.kv.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected key1 to be set from replica write")
	}
}

func TestClusterAddSlaveCommand(t *testing.T) {
	origClusterMode := clusterMode
	clusterMode = true
	defer func() { clusterMode = origClusterMode }()

	s := NewServer(":6379", ":16379")

	txn := &Transaction{pending: make(map[string]string), deleted: make(map[string]bool)}
	inTxn := false

	resp := s.processCommand("CLUSTER ADDSLAVE localhost:6380", txn, &inTxn)
	if !strings.Contains(resp, "OK") {
		t.Errorf("Expected OK for addslave, got: %s", resp)
	}

	slaves := s.cluster.replication.GetSlaveAddrs()
	if len(slaves) != 1 || slaves[0] != "localhost:6380" {
		t.Errorf("Expected slave localhost:6380, got %v", slaves)
	}
}

func TestKVGetAll(t *testing.T) {
	kv := NewMiniKV()

	kv.Set("key1", "value1")
	kv.Set("key2", "value2")
	kv.Set("key3", "value3")

	all := kv.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(all))
	}

	if all["key1"] != "value1" || all["key2"] != "value2" || all["key3"] != "value3" {
		t.Errorf("GetAll returned unexpected data")
	}
}

func TestKVSetFromMap(t *testing.T) {
	kv := NewMiniKV()

	data := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	kv.SetFromMap(data)

	if v, _ := kv.Get("a"); v != "1" {
		t.Errorf("SetFromMap did not set value for 'a' correctly, got %s", v)
	}
	if v, _ := kv.Get("b"); v != "2" {
		t.Errorf("SetFromMap did not set value for 'b' correctly, got %s", v)
	}
	if v, _ := kv.Get("c"); v != "3" {
		t.Errorf("SetFromMap did not set value for 'c' correctly, got %s", v)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
