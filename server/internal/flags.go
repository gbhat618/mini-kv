package internal

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	MaxKeySize     = DefaultMaxKeySize
	MaxValueSize   = DefaultMaxValueSize
	MaxConns       = DefaultMaxConns
	AuthPassword   string
	TLSCertFile    string
	TLSKeyFile     string
	ReadTimeout    = DefaultReadTimeout
	ClusterMode    bool
	ClusterPeers   string
	NodeID         string
	GossipPort     = 16379
	ConsolePort    = "8080"
	ConsoleEnabled bool
	ServerHost     = "localhost"
	ServerPort     = 6379
	ServerAddr     = ""
	ConfigFile     = ""
	logLevelFlag   string
)

const (
	DefaultMaxKeySize   = 256
	DefaultMaxValueSize = 1024 * 1024 * 10
	DefaultMaxConns     = 100
	DefaultReadTimeout  = 30
	DefaultServerPort   = 6379
	DefaultServerHost   = "localhost"
)

type Config struct {
	Server struct {
		Host string `ini:"host"`
		Port int    `ini:"port"`
	} `ini:"server"`
	Cluster struct {
		Enabled    bool   `ini:"enabled"`
		Peers      string `ini:"peers"`
		NodeID     string `ini:"node_id"`
		GossipPort int    `ini:"gossip_port"`
	} `ini:"cluster"`
	Security struct {
		Password string `ini:"password"`
		TLSCert  string `ini:"tls_cert"`
		TLSKey   string `ini:"tls_key"`
	} `ini:"security"`
	Limits struct {
		MaxKeySize   int `ini:"max_key_size"`
		MaxValueSize int `ini:"max_value_size"`
		MaxConns     int `ini:"max_connections"`
		ReadTimeout  int `ini:"read_timeout"`
	} `ini:"limits"`
	Console struct {
		Enabled bool   `ini:"enabled"`
		Port    string `ini:"port"`
	} `ini:"console"`
}

func LoadConfig(filename string) (*Config, error) {
	if filename == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := parseINI(string(data), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

func parseINI(data string, cfg *Config) error {
	lines := strings.Split(data, "\n")
	var currentSection string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "server":
			if key == "host" {
				cfg.Server.Host = value
			} else if key == "port" {
				if port, err := strconv.Atoi(value); err == nil {
					cfg.Server.Port = port
				}
			}
		case "cluster":
			if key == "enabled" {
				cfg.Cluster.Enabled = value == "true"
			} else if key == "peers" {
				cfg.Cluster.Peers = value
			} else if key == "node_id" {
				cfg.Cluster.NodeID = value
			} else if key == "gossip_port" {
				if port, err := strconv.Atoi(value); err == nil {
					cfg.Cluster.GossipPort = port
				}
			}
		case "security":
			if key == "password" {
				cfg.Security.Password = value
			} else if key == "tls_cert" {
				cfg.Security.TLSCert = value
			} else if key == "tls_key" {
				cfg.Security.TLSKey = value
			}
		case "limits":
			if key == "max_key_size" {
				if val, err := strconv.Atoi(value); err == nil {
					cfg.Limits.MaxKeySize = val
				}
			} else if key == "max_value_size" {
				if val, err := strconv.Atoi(value); err == nil {
					cfg.Limits.MaxValueSize = val
				}
			} else if key == "max_connections" {
				if val, err := strconv.Atoi(value); err == nil {
					cfg.Limits.MaxConns = val
				}
			} else if key == "read_timeout" {
				if val, err := strconv.Atoi(value); err == nil {
					cfg.Limits.ReadTimeout = val
				}
			}
		case "console":
			if key == "enabled" {
				cfg.Console.Enabled = value == "true"
			} else if key == "port" {
				cfg.Console.Port = value
			}
		}
	}

	return nil
}

func ApplyConfig(cfg *Config) {
	if cfg == nil {
		return
	}

	if cfg.Server.Host != "" {
		ServerHost = cfg.Server.Host
	}
	if cfg.Server.Port > 0 {
		ServerPort = cfg.Server.Port
	}
	if cfg.Server.Host != "" || cfg.Server.Port > 0 {
		ServerAddr = fmt.Sprintf("%s:%d", ServerHost, ServerPort)
	}

	if cfg.Cluster.Enabled {
		ClusterMode = true
	}
	if cfg.Cluster.Peers != "" {
		ClusterPeers = cfg.Cluster.Peers
	}
	if cfg.Cluster.NodeID != "" {
		NodeID = cfg.Cluster.NodeID
	}
	if cfg.Cluster.GossipPort > 0 {
		GossipPort = cfg.Cluster.GossipPort
	}

	if cfg.Security.Password != "" {
		AuthPassword = cfg.Security.Password
	}
	if cfg.Security.TLSCert != "" {
		TLSCertFile = cfg.Security.TLSCert
	}
	if cfg.Security.TLSKey != "" {
		TLSKeyFile = cfg.Security.TLSKey
	}

	if cfg.Limits.MaxKeySize > 0 {
		MaxKeySize = cfg.Limits.MaxKeySize
	}
	if cfg.Limits.MaxValueSize > 0 {
		MaxValueSize = cfg.Limits.MaxValueSize
	}
	if cfg.Limits.MaxConns > 0 {
		MaxConns = cfg.Limits.MaxConns
	}
	if cfg.Limits.ReadTimeout > 0 {
		ReadTimeout = cfg.Limits.ReadTimeout
	}

	if cfg.Console.Enabled {
		ConsoleEnabled = true
	}
	if cfg.Console.Port != "" {
		ConsolePort = cfg.Console.Port
	}
}

func ParseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&ConfigFile, "config", "", "Config file path (INI format)")
	flag.StringVar(&logLevelFlag, "log-level", "info", "Log level: debug, info, warn, error")
	flag.StringVar(&logLevelFlag, "l", "info", "Log level (short)")
	flag.StringVar(&ServerHost, "host", DefaultServerHost, "Server host (e.g., localhost, 0.0.0.0)")
	flag.IntVar(&ServerPort, "port", DefaultServerPort, "Server port")
	flag.IntVar(&MaxKeySize, "max-key-size", DefaultMaxKeySize, "Maximum key size in bytes")
	flag.IntVar(&MaxValueSize, "max-value-size", DefaultMaxValueSize, "Maximum value size in bytes")
	flag.IntVar(&MaxConns, "max-connections", DefaultMaxConns, "Maximum number of concurrent connections")
	flag.StringVar(&AuthPassword, "password", "", "Password for authentication (leave empty for no auth)")
	flag.StringVar(&TLSCertFile, "tls-cert", "", "TLS certificate file (leave empty for no TLS)")
	flag.StringVar(&TLSKeyFile, "tls-key", "", "TLS key file (leave empty for no TLS)")
	flag.IntVar(&ReadTimeout, "read-timeout", DefaultReadTimeout, "Connection read timeout in seconds")
	flag.BoolVar(&ClusterMode, "cluster", false, "Enable clustering mode")
	flag.StringVar(&ClusterPeers, "peers", "", "Comma-separated list of peer addresses (for cluster mode)")
	flag.StringVar(&NodeID, "node-id", "", "Unique node ID (generated if not provided)")
	flag.IntVar(&GossipPort, "gossip-port", 16379, "Port for inter-node gossip communication")
	flag.StringVar(&ConsolePort, "console-port", "8080", "Port for management console UI")
	flag.BoolVar(&ConsoleEnabled, "console", false, "Enable management console UI")

	flag.Parse()

	cfg, err := LoadConfig(ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
	}
	ApplyConfig(cfg)

	if ServerAddr == "" {
		ServerAddr = fmt.Sprintf("%s:%d", ServerHost, ServerPort)
	}

	SetLogLevel(logLevelFlag)
}

func GetAddress() string {
	if ServerAddr != "" {
		return ServerAddr
	}
	return fmt.Sprintf("%s:%d", ServerHost, ServerPort)
}

func GetGossipAddress() string {
	return fmt.Sprintf(":%d", GossipPort)
}

func GetServerHost() string {
	return ServerHost
}

func GetServerPort() int {
	return ServerPort
}

func GetConfigFile() string {
	return ConfigFile
}
