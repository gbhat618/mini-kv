package internal

import (
	"flag"
	"fmt"
	"os"
)

var (
	MaxKeySize   = DefaultMaxKeySize
	MaxValueSize = DefaultMaxValueSize
	MaxConns     = DefaultMaxConns
	AuthPassword string
	TLSCertFile  string
	TLSKeyFile   string
	ReadTimeout  = DefaultReadTimeout
	ClusterMode  bool
	ClusterPeers string
	NodeID       string
	GossipPort   = 16379
	logLevelFlag string
)

const (
	DefaultMaxKeySize   = 256
	DefaultMaxValueSize = 1024 * 1024 * 10
	DefaultMaxConns     = 100
	DefaultReadTimeout  = 30
)

func ParseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [address]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&logLevelFlag, "log-level", "info", "Log level: debug, info, warn, error")
	flag.StringVar(&logLevelFlag, "l", "info", "Log level (short)")
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

	flag.Parse()

	SetLogLevel(logLevelFlag)
}

func GetAddress() string {
	args := flag.Args()
	if len(args) > 0 {
		return args[0]
	}
	return ":6379"
}

func GetGossipAddress() string {
	return fmt.Sprintf(":%d", GossipPort)
}
