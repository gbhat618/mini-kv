package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func connect(host string, port int) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	var conn net.Conn
	var err error
	for i := 0; i < 30; i++ {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			return conn, nil
		}
		time.Sleep(time.Second)
	}
	return nil, err
}

func sendCommand(conn net.Conn, cmd string) string {
	fmt.Fprintf(conn, "%s\n", cmd)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	data, _ := io.ReadAll(conn)
	return string(data)
}

func TestPing(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	resp := sendCommand(conn, "PING")
	if !strings.Contains(resp, "PONG") {
		t.Errorf("Expected PONG, got: %s", resp)
	}
}

func TestSetGet(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "FLUSHDB")

	resp := sendCommand(conn, "SET key1 value1")
	if !strings.Contains(resp, "OK") {
		t.Errorf("SET failed, got: %s", resp)
	}

	resp = sendCommand(conn, "GET key1")
	if !strings.Contains(resp, "value1") {
		t.Errorf("GET failed, got: %s", resp)
	}
}

func TestGetNonExistent(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	resp := sendCommand(conn, "GET nonexistent")
	if !strings.Contains(resp, "-1") {
		t.Errorf("Expected -1, got: %s", resp)
	}
}

func TestMultipleKeys(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "FLUSHDB")
	sendCommand(conn, "SET name Alice")
	sendCommand(conn, "SET age 30")
	sendCommand(conn, "SET city NYC")

	resp := sendCommand(conn, "KEYS")
	if !strings.Contains(resp, "name") || !strings.Contains(resp, "age") || !strings.Contains(resp, "city") {
		t.Errorf("KEYS failed, got: %s", resp)
	}
}

func TestDelete(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "FLUSHDB")
	sendCommand(conn, "SET todelete temp")

	resp := sendCommand(conn, "DEL todelete")
	if !strings.Contains(resp, "1") {
		t.Errorf("DEL failed, got: %s", resp)
	}

	resp = sendCommand(conn, "GET todelete")
	if !strings.Contains(resp, "-1") {
		t.Errorf("Expected -1 after delete, got: %s", resp)
	}
}

func TestDeleteMultiple(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "FLUSHDB")
	sendCommand(conn, "SET key1 val1")
	sendCommand(conn, "SET key2 val2")
	sendCommand(conn, "SET key3 val3")

	resp := sendCommand(conn, "DEL key1 key2 key3")
	if !strings.Contains(resp, "3") {
		t.Errorf("DEL multiple failed, got: %s", resp)
	}
}

func TestFlushDB(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "SET temp data")
	sendCommand(conn, "SET temp2 data2")

	resp := sendCommand(conn, "FLUSHDB")
	if !strings.Contains(resp, "OK") {
		t.Errorf("FLUSHDB failed, got: %s", resp)
	}

	resp = sendCommand(conn, "KEYS")
	if !strings.Contains(resp, "*0") {
		t.Errorf("Expected empty keys after FLUSHDB, got: %s", resp)
	}
}

func TestValueWithSpaces(t *testing.T) {
	conn, err := connect("mini-kv-server", 6379)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	sendCommand(conn, "FLUSHDB")
	resp := sendCommand(conn, "SET message hello world")
	if !strings.Contains(resp, "OK") {
		t.Errorf("SET failed, got: %s", resp)
	}

	resp = sendCommand(conn, "GET message")
	if !strings.Contains(resp, "hello world") {
		t.Errorf("GET failed, got: %s", resp)
	}
}
