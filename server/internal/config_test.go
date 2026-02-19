package internal

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
[server]
host = 0.0.0.0
port = 6380

[cluster]
enabled = true
peers = localhost:6379,localhost:6380
node_id = test-node-1
gossip_port = 16380

[security]
password = secret123
tls_cert = /path/to/cert.pem
tls_key = /path/to/key.pem

[limits]
max_key_size = 512
max_value_size = 20971520
max_connections = 200
read_timeout = 60

[console]
enabled = true
port = 9090
`
	tmpFile, err := os.CreateTemp("", "config-*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 6380 {
		t.Errorf("Expected port 6380, got %d", cfg.Server.Port)
	}

	if !cfg.Cluster.Enabled {
		t.Error("Expected cluster enabled")
	}
	if cfg.Cluster.Peers != "localhost:6379,localhost:6380" {
		t.Errorf("Expected peers, got %s", cfg.Cluster.Peers)
	}
	if cfg.Cluster.NodeID != "test-node-1" {
		t.Errorf("Expected node_id test-node-1, got %s", cfg.Cluster.NodeID)
	}

	if cfg.Security.Password != "secret123" {
		t.Errorf("Expected password secret123, got %s", cfg.Security.Password)
	}

	if cfg.Limits.MaxKeySize != 512 {
		t.Errorf("Expected max_key_size 512, got %d", cfg.Limits.MaxKeySize)
	}
	if cfg.Limits.MaxValueSize != 20971520 {
		t.Errorf("Expected max_value_size 20971520, got %d", cfg.Limits.MaxValueSize)
	}
	if cfg.Limits.MaxConns != 200 {
		t.Errorf("Expected max_connections 200, got %d", cfg.Limits.MaxConns)
	}
	if cfg.Limits.ReadTimeout != 60 {
		t.Errorf("Expected read_timeout 60, got %d", cfg.Limits.ReadTimeout)
	}

	if !cfg.Console.Enabled {
		t.Error("Expected console enabled")
	}
	if cfg.Console.Port != "9090" {
		t.Errorf("Expected console port 9090, got %s", cfg.Console.Port)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/config.ini")
	if err != nil {
		t.Fatalf("Expected no error for nonexistent file, got %v", err)
	}
	if cfg != nil {
		t.Error("Expected nil config for nonexistent file")
	}
}

func TestApplyConfig(t *testing.T) {
	originalHost := ServerHost
	originalPort := ServerPort
	originalAddr := ServerAddr

	ServerHost = DefaultServerHost
	ServerPort = DefaultServerPort
	ServerAddr = ""

	cfg := &Config{
		Server: struct {
			Host string `ini:"host"`
			Port int    `ini:"port"`
		}{Host: "192.168.1.1", Port: 6380},
		Cluster: struct {
			Enabled    bool   `ini:"enabled"`
			Peers      string `ini:"peers"`
			NodeID     string `ini:"node_id"`
			GossipPort int    `ini:"gossip_port"`
		}{Enabled: true, Peers: "node1:6379", NodeID: "test-node", GossipPort: 16400},
		Security: struct {
			Password string `ini:"password"`
			TLSCert  string `ini:"tls_cert"`
			TLSKey   string `ini:"tls_key"`
		}{Password: "testpass", TLSCert: "/cert", TLSKey: "/key"},
		Limits: struct {
			MaxKeySize   int `ini:"max_key_size"`
			MaxValueSize int `ini:"max_value_size"`
			MaxConns     int `ini:"max_connections"`
			ReadTimeout  int `ini:"read_timeout"`
		}{MaxKeySize: 128, MaxValueSize: 5000000, MaxConns: 50, ReadTimeout: 15},
		Console: struct {
			Enabled bool   `ini:"enabled"`
			Port    string `ini:"port"`
		}{Enabled: true, Port: "8888"},
	}

	ApplyConfig(cfg)

	if ServerHost != "192.168.1.1" {
		t.Errorf("Expected host 192.168.1.1, got %s", ServerHost)
	}
	if ServerPort != 6380 {
		t.Errorf("Expected port 6380, got %d", ServerPort)
	}
	if ServerAddr != "192.168.1.1:6380" {
		t.Errorf("Expected addr 192.168.1.1:6380, got %s", ServerAddr)
	}

	if !ClusterMode {
		t.Error("Expected ClusterMode to be true")
	}
	if ClusterPeers != "node1:6379" {
		t.Errorf("Expected peers node1:6379, got %s", ClusterPeers)
	}
	if NodeID != "test-node" {
		t.Errorf("Expected node_id test-node, got %s", NodeID)
	}
	if GossipPort != 16400 {
		t.Errorf("Expected gossip_port 16400, got %d", GossipPort)
	}

	if AuthPassword != "testpass" {
		t.Errorf("Expected password testpass, got %s", AuthPassword)
	}
	if TLSCertFile != "/cert" {
		t.Errorf("Expected tls_cert /cert, got %s", TLSCertFile)
	}
	if TLSKeyFile != "/key" {
		t.Errorf("Expected tls_key /key, got %s", TLSKeyFile)
	}

	if MaxKeySize != 128 {
		t.Errorf("Expected max_key_size 128, got %d", MaxKeySize)
	}
	if MaxValueSize != 5000000 {
		t.Errorf("Expected max_value_size 5000000, got %d", MaxValueSize)
	}
	if MaxConns != 50 {
		t.Errorf("Expected max_connections 50, got %d", MaxConns)
	}
	if ReadTimeout != 15 {
		t.Errorf("Expected read_timeout 15, got %d", ReadTimeout)
	}

	if !ConsoleEnabled {
		t.Error("Expected ConsoleEnabled to be true")
	}
	if ConsolePort != "8888" {
		t.Errorf("Expected console port 8888, got %s", ConsolePort)
	}

	ServerHost = originalHost
	ServerPort = originalPort
	ServerAddr = originalAddr
}

func TestApplyConfigNil(t *testing.T) {
	originalHost := ServerHost

	ApplyConfig(nil)

	if ServerHost != originalHost {
		t.Errorf("Expected host to remain unchanged, got %s", ServerHost)
	}
}

func TestGetAddress(t *testing.T) {
	tests := []struct {
		host     string
		port     int
		addr     string
		expected string
	}{
		{"localhost", 6379, "", "localhost:6379"},
		{"0.0.0.0", 6380, "", "0.0.0.0:6380"},
		{"192.168.1.1", 6379, "", "192.168.1.1:6379"},
		{"localhost", 6379, "custom:6380", "custom:6380"},
	}

	for _, tt := range tests {
		ServerHost = tt.host
		ServerPort = tt.port
		ServerAddr = tt.addr

		result := GetAddress()
		if result != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, result)
		}
	}
}

func TestParseFlagsWithConfig(t *testing.T) {
	content := `
[server]
host = 0.0.0.0
port = 6380

[security]
password = configpass
`
	tmpFile, err := os.CreateTemp("", "config-*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "-config", tmpFile.Name()}
	ConfigFile = ""
	ServerHost = DefaultServerHost
	ServerPort = DefaultServerPort
	ServerAddr = ""
	AuthPassword = ""

	ParseFlags()

	if ServerHost != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", ServerHost)
	}
	if ServerPort != 6380 {
		t.Errorf("Expected port 6380, got %d", ServerPort)
	}
	if AuthPassword != "configpass" {
		t.Errorf("Expected password configpass, got %s", AuthPassword)
	}
}
