package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

type CommandProcessor struct {
	server *Server
}

func NewCommandProcessor(s *Server) *CommandProcessor {
	return &CommandProcessor{server: s}
}

func (cp *CommandProcessor) ProcessCommand(cmd string, txn *Transaction, inTxn *bool) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		Warn("Empty command received")
		return "-ERR empty command\r\n"
	}

	cmdUpper := strings.ToUpper(parts[0])

	if cmdUpper == "CLUSTER" && len(parts) > 1 {
		return cp.processClusterCommand(strings.ToUpper(parts[1]), parts[2:])
	}

	if cmdUpper == "REPLICAOF" {
		return cp.processReplicationCommand(parts[1:])
	}

	if cmdUpper == "SYNC" {
		return cp.processSyncCommand()
	}

	if cmdUpper == "REPLICA_WRITE" && len(parts) > 1 {
		return cp.processReplicaWriteCommand(parts[1])
	}

	if cmdUpper == "ROLE" {
		return cp.processRoleCommand()
	}

	if cmdUpper == "INFO" && len(parts) > 1 && strings.ToUpper(parts[1]) == "REPLICATION" {
		return cp.processReplicationInfoCommand()
	}

	if cp.server.cluster != nil && cmdUpper != "CLUSTER" && cmdUpper != "PING" &&
		cmdUpper != "AUTH" && cmdUpper != "INFO" && cmdUpper != "ROLE" &&
		cmdUpper != "SYNC" && cmdUpper != "REPLICAOF" && cmdUpper != "REPLICA_WRITE" {
		targetNode := cp.server.cluster.GetNodeForKey(strings.Join(parts, " "))
		if targetNode != cp.server.cluster.GetNodeID() && targetNode != "" {
			Debug("Forwarding command to node %s", targetNode)
			return cp.forwardToNode(cmd, targetNode)
		}
	}

	switch cmdUpper {
	case "BEGIN":
		return cp.processBegin(txn, inTxn)
	case "COMMIT":
		return cp.processCommit(txn, inTxn)
	case "ROLLBACK":
		return cp.processRollback(txn, inTxn)
	case "SET":
		return cp.processSet(parts, txn, inTxn)
	case "GET":
		return cp.processGet(parts, txn, inTxn)
	case "DEL":
		return cp.processDel(parts, txn, inTxn)
	case "KEYS":
		return cp.processKeys(txn, inTxn)
	case "FLUSHDB":
		return cp.processFlushDB(inTxn)
	case "PING":
		return "+PONG\r\n"
	case "INFO":
		return cp.processInfo()
	case "AUTH":
		return "-ERR already authenticated\r\n"
	default:
		Warn("Unknown command: %s", cmdUpper)
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmdUpper)
	}
}

func (cp *CommandProcessor) processBegin(txn *Transaction, inTxn *bool) string {
	if *inTxn {
		Warn("BEGIN: transaction already in progress")
		return "-ERR transaction already in progress\r\n"
	}
	*inTxn = true
	txn.pending = make(map[string]string)
	txn.deleted = make(map[string]bool)
	Info("BEGIN transaction")
	return "+OK\r\n"
}

func (cp *CommandProcessor) processCommit(txn *Transaction, inTxn *bool) string {
	if !*inTxn {
		Warn("COMMIT: no transaction in progress")
		return "-ERR no transaction in progress\r\n"
	}
	deletedCount := 0
	cp.server.kv.mu.Lock()
	for key, value := range txn.pending {
		cp.server.kv.store[key] = value
	}
	for key, wasDeleted := range txn.deleted {
		if wasDeleted {
			delete(cp.server.kv.store, key)
			deletedCount++
		}
	}
	cp.server.kv.mu.Unlock()
	Info("COMMIT transaction (%d keys set, %d deleted)", len(txn.pending), deletedCount)
	*inTxn = false
	txn.pending = make(map[string]string)
	txn.deleted = make(map[string]bool)
	return "+OK\r\n"
}

func (cp *CommandProcessor) processRollback(txn *Transaction, inTxn *bool) string {
	if !*inTxn {
		Warn("ROLLBACK: no transaction in progress")
		return "-ERR no transaction in progress\r\n"
	}
	Info("ROLLBACK transaction")
	*inTxn = false
	txn.pending = make(map[string]string)
	txn.deleted = make(map[string]bool)
	return "+OK\r\n"
}

func (cp *CommandProcessor) processSet(parts []string, txn *Transaction, inTxn *bool) string {
	if len(parts) < 3 {
		Warn("SET: wrong number of arguments")
		return "-ERR wrong number of arguments for 'set' command\r\n"
	}
	key := parts[1]
	value := strings.Join(parts[2:], " ")

	if len(key) > MaxKeySize {
		Warn("SET: key size %d exceeds maximum %d", len(key), MaxKeySize)
		return fmt.Sprintf("-ERR key size exceeds maximum of %d bytes\r\n", MaxKeySize)
	}
	if len(value) > MaxValueSize {
		Warn("SET: value size %d exceeds maximum %d", len(value), MaxValueSize)
		return fmt.Sprintf("-ERR value size exceeds maximum of %d bytes\r\n", MaxValueSize)
	}

	if *inTxn {
		txn.deleted[key] = false
		txn.pending[key] = value
		Info("SET (txn) %s %s", key, value)
	} else {
		cp.server.kv.Set(key, value)

		if cp.server.cluster != nil && cp.server.cluster.replication.GetRole() == RoleMaster &&
			cp.server.cluster.replication.GetSlaveAddrs() != nil {
			cp.server.cluster.ReplicateToSlaves(cp.server.kv.GetAll())
		}
	}
	return "+OK\r\n"
}

func (cp *CommandProcessor) processGet(parts []string, txn *Transaction, inTxn *bool) string {
	if len(parts) < 2 {
		Warn("GET: wrong number of arguments")
		return "-ERR wrong number of arguments for 'get' command\r\n"
	}
	key := parts[1]
	if *inTxn {
		if deleted := txn.deleted[key]; deleted {
			Info("GET (txn) %s -> (nil)", key)
			return "$-1\r\n"
		}
		if value, ok := txn.pending[key]; ok {
			Info("GET (txn) %s -> %s", key, value)
			return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
		}
	}
	if val, ok := cp.server.kv.Get(key); ok {
		Info("GET %s -> %s", key, val)
		return fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)
	}
	Info("GET %s -> (nil)", key)
	return "$-1\r\n"
}

func (cp *CommandProcessor) processDel(parts []string, txn *Transaction, inTxn *bool) string {
	if len(parts) < 2 {
		Warn("DEL: wrong number of arguments")
		return "-ERR wrong number of arguments for 'del' command\r\n"
	}
	keys := parts[1:]
	if *inTxn {
		count := 0
		for _, key := range keys {
			if _, ok := cp.server.kv.Get(key); ok || txn.pending[key] != "" || txn.deleted[key] {
				txn.deleted[key] = true
				delete(txn.pending, key)
				count++
			}
		}
		Info("DEL (txn) %v -> %d", keys, count)
		return fmt.Sprintf(":%d\r\n", count)
	}
	count := cp.server.kv.Delete(keys...)
	Info("DEL %v -> %d", keys, count)
	return fmt.Sprintf(":%d\r\n", count)
}

func (cp *CommandProcessor) processKeys(txn *Transaction, inTxn *bool) string {
	keys := cp.server.kv.Keys()
	if *inTxn {
		txnKeys := make([]string, 0, len(keys))
		for _, k := range keys {
			if !txn.deleted[k] {
				txnKeys = append(txnKeys, k)
			}
		}
		for k := range txn.pending {
			txnKeys = append(txnKeys, k)
		}
		Info("KEYS (txn) -> %d keys", len(txnKeys))
		if len(txnKeys) == 0 {
			return "*0\r\n"
		}
		result := fmt.Sprintf("*%d\r\n", len(txnKeys))
		for _, k := range txnKeys {
			result += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
		}
		return result
	}
	Info("KEYS -> %d keys", len(keys))
	if len(keys) == 0 {
		return "*0\r\n"
	}
	result := fmt.Sprintf("*%d\r\n", len(keys))
	for _, k := range keys {
		result += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
	}
	return result
}

func (cp *CommandProcessor) processFlushDB(inTxn *bool) string {
	if *inTxn {
		Warn("FLUSHDB: cannot flush within transaction")
		return "-ERR cannot FLUSHDB within a transaction\r\n"
	}
	cp.server.kv.FlushDB()
	Info("FLUSHDB")
	return "+OK\r\n"
}

func (cp *CommandProcessor) processInfo() string {
	info := fmt.Sprintf("mini-kv server\r\n")
	info += fmt.Sprintf("cluster_mode: %v\r\n", cp.server.cluster != nil)
	if cp.server.cluster != nil {
		info += fmt.Sprintf("cluster_nodes: %d\r\n", len(cp.server.cluster.GetMembers()))
		info += fmt.Sprintf("cluster_node_id: %s\r\n", cp.server.cluster.GetNodeID())
		info += fmt.Sprintf("role: %s\r\n", cp.server.cluster.replication.GetRole().String())
	}
	return fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)
}

func (cp *CommandProcessor) forwardToNode(cmd string, nodeID string) string {
	addr := cp.server.cluster.GetNodeAddr(nodeID)
	if addr == "" {
		return "-ERR node not found\r\n"
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Sprintf("-ERR failed to connect to node: %v\r\n", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", cmd)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Sprintf("-ERR failed to read response: %v\r\n", err)
	}

	return response
}

func (cp *CommandProcessor) processReplicationCommand(args []string) string {
	if len(args) < 2 {
		return "-ERR wrong number of arguments for 'replicaof' command\r\n"
	}

	host := args[0]
	port := args[1]

	if host == "NO" && port == "ONE" {
		Info("Replication: Promoting to master")
		cp.server.cluster.replication.SetMaster("")
		return "+OK\r\n"
	}

	masterAddr := fmt.Sprintf("%s:%s", host, port)

	if port == "0" {
		Info("Replication: Promoting to master")
		cp.server.cluster.replication.SetMaster("")
		return "+OK\r\n"
	}

	Info("Replication: Becoming slave of %s", masterAddr)
	cp.server.cluster.replication.SetMaster(masterAddr)

	go func() {
		for {
			if err := cp.server.cluster.SyncFromMaster(masterAddr); err != nil {
				Warn("Replication: Sync failed: %v, retrying...", err)
				time.Sleep(5 * time.Second)
			} else {
				break
			}
		}
	}()

	return "+OK\r\n"
}

func (cp *CommandProcessor) processSyncCommand() string {
	data := cp.server.kv.GetAll()
	dataJSON, _ := json.Marshal(data)
	return fmt.Sprintf("+%s\r\n", dataJSON)
}

func (cp *CommandProcessor) processReplicaWriteCommand(dataJSON string) string {
	if cp.server.cluster.replication.GetRole() != RoleSlave {
		return "-ERR not a slave\r\n"
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return "-ERR invalid data\r\n"
	}

	cp.server.kv.SetFromMap(data)
	return "+OK\r\n"
}

func (cp *CommandProcessor) processRoleCommand() string {
	role := cp.server.cluster.replication.GetRole()
	masterAddr := cp.server.cluster.replication.GetMasterAddr()
	slaves := cp.server.cluster.replication.GetSlaveAddrs()

	result := fmt.Sprintf("role: %s\r\n", role.String())
	if role == RoleSlave {
		result += fmt.Sprintf("master: %s\r\n", masterAddr)
	} else if role == RoleMaster {
		result += fmt.Sprintf("connected_slaves: %d\r\n", len(slaves))
	}

	return fmt.Sprintf("$%d\r\n%s\r\n", len(result), result)
}

func (cp *CommandProcessor) processReplicationInfoCommand() string {
	role := cp.server.cluster.replication.GetRole()
	masterAddr := cp.server.cluster.replication.GetMasterAddr()
	slaves := cp.server.cluster.replication.GetSlaveAddrs()

	info := fmt.Sprintf("role: %s\r\n", role.String())
	if role == RoleSlave {
		info += fmt.Sprintf("master_link_status: up\r\n")
		info += fmt.Sprintf("master_host: %s\r\n", masterAddr)
	} else {
		info += fmt.Sprintf("connected_slaves: %d\r\n", len(slaves))
		for i, slave := range slaves {
			info += fmt.Sprintf("slave_%d: %s\r\n", i, slave)
		}
	}

	return fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)
}

func (cp *CommandProcessor) processClusterCommand(subCmd string, args []string) string {
	if cp.server.cluster == nil {
		return "-ERR cluster mode not enabled\r\n"
	}

	switch subCmd {
	case "INFO":
		members := cp.server.cluster.GetMembers()
		info := fmt.Sprintf("cluster_enabled: true\r\n")
		info += fmt.Sprintf("cluster_node_id: %s\r\n", cp.server.cluster.GetNodeID())
		info += fmt.Sprintf("cluster_nodes: %d\r\n", len(members))
		info += fmt.Sprintf("cluster_my_role: %s\r\n", cp.server.cluster.replication.GetRole().String())
		for _, m := range members {
			healthStr := "healthy"
			if !m.Health {
				healthStr = "unhealthy"
			}
			info += fmt.Sprintf("cluster_node: %s %s %s %s\r\n", m.ID, m.Addr, m.Role.String(), healthStr)
		}
		return fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)

	case "MEMBERS":
		members := cp.server.cluster.GetMembers()
		if len(members) == 0 {
			return "*0\r\n"
		}
		result := fmt.Sprintf("*%d\r\n", len(members))
		for _, m := range members {
			nodeInfo := fmt.Sprintf("%s %s %s", m.ID, m.Addr, m.Role.String())
			result += fmt.Sprintf("$%d\r\n%s\r\n", len(nodeInfo), nodeInfo)
		}
		return result

	case "JOIN":
		if len(args) < 1 {
			return "-ERR wrong number of arguments for 'cluster join' command\r\n"
		}
		peerAddr := args[0]
		cp.server.cluster.StartGossip([]string{peerAddr})
		Info("Cluster: Joined peer %s", peerAddr)
		return "+OK\r\n"

	case "ADDSLAVE":
		if len(args) < 1 {
			return "-ERR wrong number of arguments for 'cluster addslave' command\r\n"
		}
		slaveAddr := args[0]
		cp.server.cluster.replication.AddSlave(slaveAddr)
		return "+OK\r\n"

	default:
		return fmt.Sprintf("-ERR unknown cluster command '%s'\r\n", subCmd)
	}
}
