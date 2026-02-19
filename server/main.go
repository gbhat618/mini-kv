package main

import (
	"bufio"
	"crypto/subtle"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

const (
	DefaultMaxKeySize   = 256
	DefaultMaxValueSize = 1024 * 1024 * 10
	DefaultMaxConns     = 100
	DefaultReadTimeout  = 30 * time.Second
	HashReplicas        = 150
	GossipInterval      = 1 * time.Second
	GossipTimeout       = 5 * time.Second
)

var (
	logLevel     LogLevel
	logger       *log.Logger
	logPrefix    string
	maxKeySize   int
	maxValueSize int
	maxConns     int
	authPassword string
	tlsCertFile  string
	tlsKeyFile   string
	readTimeout  time.Duration
	clusterMode  bool
	clusterPeers string
	nodeID       string
	gossipPort   int
)

func init() {
	flag.StringVar(&logPrefix, "log-level", "info", "Log level: debug, info, warn, error")
	flag.StringVar(&logPrefix, "l", "info", "Log level (short)")
	flag.IntVar(&maxKeySize, "max-key-size", DefaultMaxKeySize, "Maximum key size in bytes")
	flag.IntVar(&maxValueSize, "max-value-size", DefaultMaxValueSize, "Maximum value size in bytes")
	flag.IntVar(&maxConns, "max-connections", DefaultMaxConns, "Maximum number of concurrent connections")
	flag.StringVar(&authPassword, "password", "", "Password for authentication (leave empty for no auth)")
	flag.StringVar(&tlsCertFile, "tls-cert", "", "TLS certificate file (leave empty for no TLS)")
	flag.StringVar(&tlsKeyFile, "tls-key", "", "TLS key file (leave empty for no TLS)")
	flag.DurationVar(&readTimeout, "read-timeout", DefaultReadTimeout, "Connection read timeout")
	flag.BoolVar(&clusterMode, "cluster", false, "Enable clustering mode")
	flag.StringVar(&clusterPeers, "peers", "", "Comma-separated list of peer addresses (for cluster mode)")
	flag.StringVar(&nodeID, "node-id", "", "Unique node ID (generated if not provided)")
	flag.IntVar(&gossipPort, "gossip-port", 16379, "Port for inter-node gossip communication")
	logger = log.New(os.Stdout, "", 0)
}

func parseFlags() {
	flag.Parse()

	switch strings.ToLower(logPrefix) {
	case "debug":
		logLevel = DEBUG
	case "info":
		logLevel = INFO
	case "warn", "warning":
		logLevel = WARN
	case "error":
		logLevel = ERROR
	default:
		logLevel = INFO
	}

	logger = log.New(os.Stdout, "", 0)
}

func debug(format string, v ...interface{}) {
	if logLevel <= DEBUG {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

func info(format string, v ...interface{}) {
	if logLevel <= INFO {
		logger.Printf("[INFO] "+format, v...)
	}
}

func warn(format string, v ...interface{}) {
	if logLevel <= WARN {
		logger.Printf("[WARN] "+format, v...)
	}
}

func logError(format string, v ...interface{}) {
	if logLevel <= ERROR {
		logger.Printf("[ERROR] "+format, v...)
	}
}

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
	debug("SET key=%q value=%q", key, value)
}

func (kv *MiniKV) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	val, ok := kv.store[key]
	if ok {
		debug("GET key=%q value=%q", key, val)
	} else {
		debug("GET key=%q not found", key)
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
			debug("DEL key=%q", key)
		}
	}
	debug("DEL deleted %d keys", count)
	return count
}

func (kv *MiniKV) Keys() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	keys := make([]string, 0, len(kv.store))
	for k := range kv.store {
		keys = append(keys, k)
	}
	debug("KEYS returned %d keys", len(keys))
	return keys
}

func (kv *MiniKV) FlushDB() {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	count := len(kv.store)
	kv.store = make(map[string]string)
	info("FLUSHDB deleted %d keys", count)
}

type Transaction struct {
	pending map[string]string
	deleted map[string]bool
}

type NodeInfo struct {
	ID        string
	Addr      string
	Timestamp int64
}

type Cluster struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeInfo
	hashRing     *ConsistentHash
	nodeID       string
	selfAddr     string
	gossipAddr   string
	gossipTicker *time.Ticker
	shutdownChan chan struct{}
}

type ConsistentHash struct {
	mu         sync.RWMutex
	replicas   int
	ring       map[uint32]string
	sortedKeys []uint32
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas:   replicas,
		ring:       make(map[uint32]string),
		sortedKeys: make([]uint32, 0),
	}
}

func (ch *ConsistentHash) AddNode(nodeID, addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := fmt.Sprintf("%s-%d-%s", nodeID, i, addr)
		hash := crc32.ChecksumIEEE([]byte(key))
		ch.ring[hash] = nodeID
		ch.sortedKeys = append(ch.sortedKeys, hash)
	}

	for i := 0; i < len(ch.sortedKeys)-1; i++ {
		for j := i + 1; j < len(ch.sortedKeys); j++ {
			if ch.sortedKeys[i] > ch.sortedKeys[j] {
				ch.sortedKeys[i], ch.sortedKeys[j] = ch.sortedKeys[j], ch.sortedKeys[i]
			}
		}
	}
}

func (ch *ConsistentHash) RemoveNode(nodeID string, addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := fmt.Sprintf("%s-%d-%s", nodeID, i, addr)
		hash := crc32.ChecksumIEEE([]byte(key))
		delete(ch.ring, hash)
	}

	ch.sortedKeys = make([]uint32, 0)
	for hash := range ch.ring {
		ch.sortedKeys = append(ch.sortedKeys, hash)
	}

	for i := 0; i < len(ch.sortedKeys)-1; i++ {
		for j := i + 1; j < len(ch.sortedKeys); j++ {
			if ch.sortedKeys[i] > ch.sortedKeys[j] {
				ch.sortedKeys[i], ch.sortedKeys[j] = ch.sortedKeys[j], ch.sortedKeys[i]
			}
		}
	}
}

func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	hash := crc32.ChecksumIEEE([]byte(key))
	idx := 0
	for i, k := range ch.sortedKeys {
		if hash <= k {
			idx = i
			break
		}
		if i == len(ch.sortedKeys)-1 {
			idx = 0
			break
		}
	}

	return ch.ring[ch.sortedKeys[idx]]
}

func (ch *ConsistentHash) GetAllNodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	nodes := make(map[string]bool)
	for _, nodeID := range ch.ring {
		nodes[nodeID] = true
	}

	result := make([]string, 0, len(nodes))
	for nodeID := range nodes {
		result = append(result, nodeID)
	}
	return result
}

func NewCluster(nodeID, selfAddr, gossipAddr string) *Cluster {
	return &Cluster{
		nodes:        make(map[string]*NodeInfo),
		hashRing:     NewConsistentHash(HashReplicas),
		nodeID:       nodeID,
		selfAddr:     selfAddr,
		gossipAddr:   gossipAddr,
		gossipTicker: time.NewTicker(GossipInterval),
		shutdownChan: make(chan struct{}),
	}
}

func (c *Cluster) AddNode(nodeID, addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodes[nodeID] = &NodeInfo{
		ID:        nodeID,
		Addr:      addr,
		Timestamp: time.Now().Unix(),
	}
	c.hashRing.AddNode(nodeID, addr)
	info("Cluster: Added node %s at %s", nodeID, addr)
}

func (c *Cluster) RemoveNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.nodes[nodeID]; ok {
		c.hashRing.RemoveNode(nodeID, node.Addr)
		delete(c.nodes, nodeID)
		info("Cluster: Removed node %s", nodeID)
	}
}

func (c *Cluster) GetNodeForKey(key string) string {
	return c.hashRing.GetNode(key)
}

func (c *Cluster) GetMembers() []*NodeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	members := make([]*NodeInfo, 0, len(c.nodes))
	for _, node := range c.nodes {
		members = append(members, node)
	}
	return members
}

func (c *Cluster) GetNodeAddr(nodeID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if node, ok := c.nodes[nodeID]; ok {
		return node.Addr
	}
	return ""
}

func (c *Cluster) GetNodeID() string {
	return c.nodeID
}

func (c *Cluster) StartGossip(peers []string) {
	if len(peers) == 0 {
		info("Cluster: No peers to gossip with")
		return
	}

	for _, peer := range peers {
		go c.gossipWithNode(peer)
	}
}

func (c *Cluster) gossipWithNode(peerAddr string) {
	for {
		select {
		case <-c.shutdownChan:
			return
		case <-c.gossipTicker.C:
			c.sendGossip(peerAddr)
		}
	}
}

func (c *Cluster) sendGossip(peerAddr string) {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		debug("Cluster: Failed to connect to peer %s: %v", peerAddr, err)
		return
	}
	defer conn.Close()

	c.mu.RLock()
	nodesJSON, _ := json.Marshal(c.nodes)
	c.mu.RUnlock()

	msg := fmt.Sprintf("CLUSTER_GOSSIP %s %s\n", c.nodeID, nodesJSON)
	conn.Write([]byte(msg))

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		debug("Cluster: Failed to read gossip response from %s: %v", peerAddr, err)
		return
	}

	if strings.HasPrefix(response, "+CLUSTER_GOSSIP") {
		parts := strings.Fields(response)
		if len(parts) >= 3 {
			var peerNodes map[string]*NodeInfo
			if err := json.Unmarshal([]byte(parts[2]), &peerNodes); err == nil {
				c.mergeNodes(peerNodes)
			}
		}
	}
}

func (c *Cluster) mergeNodes(peerNodes map[string]*NodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for nodeID, node := range peerNodes {
		if existing, ok := c.nodes[nodeID]; !ok || node.Timestamp > existing.Timestamp {
			c.nodes[nodeID] = node
			c.hashRing.AddNode(nodeID, node.Addr)
		}
	}
}

func (c *Cluster) Shutdown() {
	close(c.shutdownChan)
	c.gossipTicker.Stop()
}

type Server struct {
	addr         string
	kv           *MiniKV
	tlsConfig    *tls.Config
	connCount    atomic.Int32
	shutdownChan chan struct{}
	cluster      *Cluster
}

func NewServer(addr string, gossipAddr string) *Server {
	var cluster *Cluster
	if clusterMode {
		cluster = NewCluster(nodeID, addr, gossipAddr)
	}

	return &Server{
		addr:         addr,
		kv:           NewMiniKV(),
		shutdownChan: make(chan struct{}),
		cluster:      cluster,
	}
}

func (s *Server) SetTLSConfig(cfg *tls.Config) {
	s.tlsConfig = cfg
}

func (s *Server) Start() error {
	var ln net.Listener
	var err error

	if s.tlsConfig != nil {
		ln, err = tls.Listen("tcp", s.addr, s.tlsConfig)
		if err != nil {
			return err
		}
		info("Mini-KV Server started on %s with TLS (log level: %s)", s.addr, strings.ToUpper(logPrefix))
	} else {
		ln, err = net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
		info("Mini-KV Server started on %s (log level: %s)", s.addr, strings.ToUpper(logPrefix))
	}

	if authPassword != "" {
		info("Authentication enabled")
	}
	if s.tlsConfig != nil {
		info("TLS enabled")
	}
	info("Max connections: %d, Max key size: %d, Max value size: %d", maxConns, maxKeySize, maxValueSize)

	if s.cluster != nil {
		info("Cluster mode enabled")
		s.cluster.AddNode(s.cluster.nodeID, s.addr)

		if clusterPeers != "" {
			peers := strings.Split(clusterPeers, ",")
			for i := range peers {
				peers[i] = strings.TrimSpace(peers[i])
			}
			info("Cluster: Connecting to peers: %v", peers)
			s.cluster.StartGossip(peers)
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		logError("Shutdown signal received")
		if s.cluster != nil {
			s.cluster.Shutdown()
		}
		close(s.shutdownChan)
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.shutdownChan:
				return nil
			default:
				logError("Accept error: %v", err)
				continue
			}
		}

		if s.connCount.Load() >= int32(maxConns) {
			logError("Connection limit reached, rejecting connection from %s", conn.RemoteAddr())
			conn.Close()
			continue
		}

		s.connCount.Add(1)
		debug("Accepted connection from %s (active: %d)", conn.RemoteAddr(), s.connCount.Load())
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		s.connCount.Add(-1)
		conn.Close()
		debug("Connection closed from %s", conn.RemoteAddr())
	}()

	if readTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
	}

	reader := bufio.NewReader(conn)
	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false
	authenticated := authPassword == ""

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		debug("Received command: %q", line)

		if !authenticated {
			response := s.processAuth(line)
			conn.Write([]byte(response))
			continue
		}

		response := s.processCommand(line, txn, &inTxn)
		conn.Write([]byte(response))

		if readTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(readTimeout))
		}
	}
}

func (s *Server) processAuth(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 || strings.ToUpper(parts[0]) != "AUTH" {
		return "-ERR authentication required\r\n"
	}

	inputPassword := parts[1]
	if subtle.ConstantTimeCompare([]byte(authPassword), []byte(inputPassword)) == 1 {
		return "+OK\r\n"
	}

	logError("Failed authentication attempt")
	return "-ERR invalid password\r\n"
}

func (s *Server) forwardToNode(cmd string, nodeID string) string {
	addr := s.cluster.GetNodeAddr(nodeID)
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

func (s *Server) processCommand(cmd string, txn *Transaction, inTxn *bool) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		warn("Empty command received")
		return "-ERR empty command\r\n"
	}

	cmdUpper := strings.ToUpper(parts[0])

	if cmdUpper == "CLUSTER" && len(parts) > 1 {
		return s.processClusterCommand(strings.ToUpper(parts[1]), parts[2:])
	}

	if s.cluster != nil && cmdUpper != "CLUSTER" && cmdUpper != "PING" && cmdUpper != "AUTH" && cmdUpper != "INFO" {
		targetNode := s.cluster.GetNodeForKey(strings.Join(parts, " "))
		if targetNode != s.cluster.GetNodeID() && targetNode != "" {
			debug("Forwarding command to node %s", targetNode)
			return s.forwardToNode(cmd, targetNode)
		}
	}

	switch cmdUpper {
	case "BEGIN":
		if *inTxn {
			warn("BEGIN: transaction already in progress")
			return "-ERR transaction already in progress\r\n"
		}
		*inTxn = true
		txn.pending = make(map[string]string)
		txn.deleted = make(map[string]bool)
		info("BEGIN transaction")
		return "+OK\r\n"

	case "COMMIT":
		if !*inTxn {
			warn("COMMIT: no transaction in progress")
			return "-ERR no transaction in progress\r\n"
		}
		deletedCount := 0
		s.kv.mu.Lock()
		for key, value := range txn.pending {
			s.kv.store[key] = value
		}
		for key, wasDeleted := range txn.deleted {
			if wasDeleted {
				delete(s.kv.store, key)
				deletedCount++
			}
		}
		s.kv.mu.Unlock()
		info("COMMIT transaction (%d keys set, %d deleted)", len(txn.pending), deletedCount)
		*inTxn = false
		txn.pending = make(map[string]string)
		txn.deleted = make(map[string]bool)
		return "+OK\r\n"

	case "ROLLBACK":
		if !*inTxn {
			warn("ROLLBACK: no transaction in progress")
			return "-ERR no transaction in progress\r\n"
		}
		info("ROLLBACK transaction")
		*inTxn = false
		txn.pending = make(map[string]string)
		txn.deleted = make(map[string]bool)
		return "+OK\r\n"

	case "SET":
		if len(parts) < 3 {
			warn("SET: wrong number of arguments")
			return "-ERR wrong number of arguments for 'set' command\r\n"
		}
		key := parts[1]
		value := strings.Join(parts[2:], " ")

		if len(key) > maxKeySize {
			warn("SET: key size %d exceeds maximum %d", len(key), maxKeySize)
			return fmt.Sprintf("-ERR key size exceeds maximum of %d bytes\r\n", maxKeySize)
		}
		if len(value) > maxValueSize {
			warn("SET: value size %d exceeds maximum %d", len(value), maxValueSize)
			return fmt.Sprintf("-ERR value size exceeds maximum of %d bytes\r\n", maxValueSize)
		}

		if *inTxn {
			txn.deleted[key] = false
			txn.pending[key] = value
			info("SET (txn) %s %s", key, value)
		} else {
			s.kv.Set(key, value)
		}
		return "+OK\r\n"

	case "GET":
		if len(parts) < 2 {
			warn("GET: wrong number of arguments")
			return "-ERR wrong number of arguments for 'get' command\r\n"
		}
		key := parts[1]
		if *inTxn {
			if deleted := txn.deleted[key]; deleted {
				info("GET (txn) %s -> (nil)", key)
				return "$-1\r\n"
			}
			if value, ok := txn.pending[key]; ok {
				info("GET (txn) %s -> %s", key, value)
				return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
			}
		}
		if val, ok := s.kv.Get(key); ok {
			info("GET %s -> %s", key, val)
			return fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)
		}
		info("GET %s -> (nil)", key)
		return "$-1\r\n"

	case "DEL":
		if len(parts) < 2 {
			warn("DEL: wrong number of arguments")
			return "-ERR wrong number of arguments for 'del' command\r\n"
		}
		keys := parts[1:]
		if *inTxn {
			count := 0
			for _, key := range keys {
				if _, ok := s.kv.Get(key); ok || txn.pending[key] != "" || txn.deleted[key] {
					txn.deleted[key] = true
					delete(txn.pending, key)
					count++
				}
			}
			info("DEL (txn) %v -> %d", keys, count)
			return fmt.Sprintf(":%d\r\n", count)
		}
		count := s.kv.Delete(keys...)
		info("DEL %v -> %d", keys, count)
		return fmt.Sprintf(":%d\r\n", count)

	case "KEYS":
		keys := s.kv.Keys()
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
			info("KEYS (txn) -> %d keys", len(txnKeys))
			if len(txnKeys) == 0 {
				return "*0\r\n"
			}
			result := fmt.Sprintf("*%d\r\n", len(txnKeys))
			for _, k := range txnKeys {
				result += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
			}
			return result
		}
		info("KEYS -> %d keys", len(keys))
		if len(keys) == 0 {
			return "*0\r\n"
		}
		result := fmt.Sprintf("*%d\r\n", len(keys))
		for _, k := range keys {
			result += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
		}
		return result

	case "FLUSHDB":
		if *inTxn {
			warn("FLUSHDB: cannot flush within transaction")
			return "-ERR cannot FLUSHDB within a transaction\r\n"
		}
		s.kv.FlushDB()
		info("FLUSHDB")
		return "+OK\r\n"

	case "PING":
		debug("PING")
		return "+PONG\r\n"

	case "INFO":
		info := fmt.Sprintf("mini-kv server\r\n")
		info += fmt.Sprintf("cluster_mode: %v\r\n", s.cluster != nil)
		if s.cluster != nil {
			info += fmt.Sprintf("cluster_nodes: %d\r\n", len(s.cluster.GetMembers()))
			info += fmt.Sprintf("cluster_node_id: %s\r\n", s.cluster.GetNodeID())
		}
		return fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)

	case "AUTH":
		warn("AUTH: already authenticated")
		return "-ERR already authenticated\r\n"

	default:
		warn("Unknown command: %s", cmdUpper)
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmdUpper)
	}
}

func (s *Server) processClusterCommand(subCmd string, args []string) string {
	if s.cluster == nil {
		return "-ERR cluster mode not enabled\r\n"
	}

	switch subCmd {
	case "INFO":
		members := s.cluster.GetMembers()
		info := fmt.Sprintf("cluster_enabled: true\r\n")
		info += fmt.Sprintf("cluster_node_id: %s\r\n", s.cluster.GetNodeID())
		info += fmt.Sprintf("cluster_nodes: %d\r\n", len(members))
		for _, m := range members {
			info += fmt.Sprintf("cluster_node: %s %s\r\n", m.ID, m.Addr)
		}
		return fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)

	case "MEMBERS":
		members := s.cluster.GetMembers()
		if len(members) == 0 {
			return "*0\r\n"
		}
		result := fmt.Sprintf("*%d\r\n", len(members))
		for _, m := range members {
			nodeInfo := fmt.Sprintf("%s %s", m.ID, m.Addr)
			result += fmt.Sprintf("$%d\r\n%s\r\n", len(nodeInfo), nodeInfo)
		}
		return result

	case "JOIN":
		if len(args) < 1 {
			return "-ERR wrong number of arguments for 'cluster join' command\r\n"
		}
		peerAddr := args[0]
		s.cluster.StartGossip([]string{peerAddr})
		info("Cluster: Joined peer %s", peerAddr)
		return "+OK\r\n"

	default:
		return fmt.Sprintf("-ERR unknown cluster command '%s'\r\n", subCmd)
	}
}

func generateNodeID() string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(time.Now().UnixNano()))
	return fmt.Sprintf("node-%x", b)
}

func main() {
	parseFlags()

	if nodeID == "" {
		nodeID = generateNodeID()
	}

	addr := ":6379"
	gossipAddr := ":16379"
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		addr = args[0]
	}

	server := NewServer(addr, gossipAddr)

	if tlsCertFile != "" && tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate: %v", err)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		server.SetTLSConfig(tlsConfig)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
