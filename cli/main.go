package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultHost = "localhost"
	DefaultPort = 6379
)

var (
	host          string
	port          int
	tlsFlag       bool
	tlsSkipVerify bool
	password      string
	configFile    string
)

type CLIConfig struct {
	Server struct {
		Host string `ini:"host"`
		Port int    `ini:"port"`
	} `ini:"server"`
	Security struct {
		Password string `ini:"password"`
	} `ini:"security"`
	TLS struct {
		Enabled    bool `ini:"enabled"`
		SkipVerify bool `ini:"skip_verify"`
	} `ini:"tls"`
}

func loadConfig(filename string) (*CLIConfig, error) {
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

	cfg := &CLIConfig{}
	if err := parseINI(string(data), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

func parseINI(data string, cfg *CLIConfig) error {
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
		case "security":
			if key == "password" {
				cfg.Security.Password = value
			}
		case "tls":
			if key == "enabled" {
				cfg.TLS.Enabled = value == "true"
			} else if key == "skip_verify" {
				cfg.TLS.SkipVerify = value == "true"
			}
		}
	}

	return nil
}

func applyConfig(cfg *CLIConfig) {
	if cfg == nil {
		return
	}

	if cfg.Server.Host != "" {
		host = cfg.Server.Host
	}
	if cfg.Server.Port > 0 {
		port = cfg.Server.Port
	}

	if cfg.Security.Password != "" {
		password = cfg.Security.Password
	}

	if cfg.TLS.Enabled {
		tlsFlag = true
	}
	if cfg.TLS.SkipVerify {
		tlsSkipVerify = true
	}
}

func parseFlags() {
	flag.StringVar(&configFile, "config", "", "Config file path (INI format)")
	flag.StringVar(&host, "host", DefaultHost, "Server hostname or IP address")
	flag.StringVar(&host, "h", DefaultHost, "Server hostname or IP address (short)")
	flag.IntVar(&port, "port", DefaultPort, "Server port")
	flag.IntVar(&port, "p", DefaultPort, "Server port (short)")
	flag.StringVar(&password, "password", "", "Password for authentication")
	flag.BoolVar(&tlsFlag, "tls", false, "Use TLS connection")
	flag.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  SET <key> <value>   Set a key-value pair\n")
		fmt.Fprintf(os.Stderr, "  GET <key>           Get a value by key\n")
		fmt.Fprintf(os.Stderr, "  DEL <key> [key...]  Delete one or more keys\n")
		fmt.Fprintf(os.Stderr, "  KEYS                List all keys\n")
		fmt.Fprintf(os.Stderr, "  FLUSHDB             Delete all keys\n")
		fmt.Fprintf(os.Stderr, "  BEGIN               Start a transaction\n")
		fmt.Fprintf(os.Stderr, "  COMMIT              Commit a transaction\n")
		fmt.Fprintf(os.Stderr, "  ROLLBACK            Rollback a transaction\n")
		fmt.Fprintf(os.Stderr, "  PING                Ping the server\n")
		fmt.Fprintf(os.Stderr, "  AUTH <password>     Authenticate with server\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s PING\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --host localhost --port 6379 GET mykey\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --config /etc/mini-kv.ini SET key value\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.100 -p 6380 GET key\n", os.Args[0])
	}
	flag.Parse()

	cfg, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
	}
	applyConfig(cfg)
}

func main() {
	parseFlags()

	args := flag.Args()
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--host" && i+1 < len(args) {
			host = args[i+1]
			i += 2
		} else if arg == "--port" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i += 2
		} else if arg == "-h" && i+1 < len(args) {
			host = args[i+1]
			i += 2
		} else if arg == "-p" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i += 2
		} else {
			break
		}
	}

	command := strings.Join(args[i:], " ")
	if command == "" {
		fmt.Fprintln(os.Stderr, "Error: no command provided")
		os.Exit(1)
	}

	address := fmt.Sprintf("%s:%d", host, port)

	var conn net.Conn
	var err error

	if tlsFlag {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		}
		conn, err = tls.Dial("tcp", address, tlsConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
			os.Exit(1)
		}
	} else {
		conn, err = net.Dial("tcp", address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
			os.Exit(1)
		}
	}
	defer conn.Close()

	if password != "" {
		authCmd := fmt.Sprintf("AUTH %s", password)
		fmt.Fprintf(conn, "%s\n", authCmd)

		reader := bufio.NewReader(conn)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading auth response: %v\n", err)
			os.Exit(1)
		}

		if !strings.HasPrefix(response, "+OK") {
			fmt.Fprintf(os.Stderr, "Authentication failed: %s", response)
			os.Exit(1)
		}
	}

	fmt.Fprintf(conn, "%s\n", command)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(response)
}
