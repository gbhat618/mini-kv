package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	logLevel  LogLevel
	logger    *log.Logger
	logPrefix string
)

func init() {
	flag.StringVar(&logPrefix, "log-level", "info", "Log level: debug, info, warn, error")
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

type Server struct {
	addr string
	kv   *MiniKV
}

func NewServer(addr string) *Server {
	return &Server{addr: addr, kv: NewMiniKV()}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	info("Mini-KV Server started on %s (log level: %s)", s.addr, strings.ToUpper(logPrefix))

	for {
		conn, err := ln.Accept()
		if err != nil {
			logError("Accept error: %v", err)
			continue
		}
		debug("Accepted connection from %s", conn.RemoteAddr())
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	txn := &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
	inTxn := false

	defer func() {
		conn.Close()
		debug("Connection closed from %s", conn.RemoteAddr())
	}()

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
		response := s.processCommand(line, txn, &inTxn)
		conn.Write([]byte(response))
	}
}

func (s *Server) processCommand(cmd string, txn *Transaction, inTxn *bool) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		warn("Empty command received")
		return "-ERR empty command\r\n"
	}

	cmdUpper := strings.ToUpper(parts[0])

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

	default:
		warn("Unknown command: %s", cmdUpper)
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmdUpper)
	}
}

func main() {
	parseFlags()
	addr := ":6379"
	server := NewServer(addr)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
