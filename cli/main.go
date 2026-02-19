package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	tlsFlag       bool
	tlsSkipVerify bool
	password      string
)

func init() {
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
	}

	flag.StringVar(&password, "password", "", "Password for authentication")
	flag.BoolVar(&tlsFlag, "tls", false, "Use TLS connection")
	flag.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification")
	flag.Parse()
}

func main() {
	host := "localhost"
	port := 6379

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
