package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

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
}

func (kv *MiniKV) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	val, ok := kv.store[key]
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
		}
	}
	return count
}

func (kv *MiniKV) Keys() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	keys := make([]string, 0, len(kv.store))
	for k := range kv.store {
		keys = append(keys, k)
	}
	return keys
}

func (kv *MiniKV) FlushDB() {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store = make(map[string]string)
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
	log.Printf("Mini-KV Server started on %s", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		response := s.processCommand(line)
		conn.Write([]byte(response))
	}
}

func (s *Server) processCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "-ERR empty command\r\n"
	}

	cmdUpper := strings.ToUpper(parts[0])

	switch cmdUpper {
	case "SET":
		if len(parts) < 3 {
			return "-ERR wrong number of arguments for 'set' command\r\n"
		}
		key := parts[1]
		value := strings.Join(parts[2:], " ")
		s.kv.Set(key, value)
		return "+OK\r\n"

	case "GET":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments for 'get' command\r\n"
		}
		key := parts[1]
		if val, ok := s.kv.Get(key); ok {
			return fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)
		}
		return "$-1\r\n"

	case "DEL":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments for 'del' command\r\n"
		}
		keys := parts[1:]
		count := s.kv.Delete(keys...)
		return fmt.Sprintf(":%d\r\n", count)

	case "KEYS":
		keys := s.kv.Keys()
		if len(keys) == 0 {
			return "*0\r\n"
		}
		result := fmt.Sprintf("*%d\r\n", len(keys))
		for _, k := range keys {
			result += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
		}
		return result

	case "PING":
		return "+PONG\r\n"

	case "FLUSHDB":
		s.kv.FlushDB()
		return "+OK\r\n"

	default:
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmdUpper)
	}
}

func main() {
	addr := ":6379"
	server := NewServer(addr)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
