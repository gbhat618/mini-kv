package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	host := "localhost"
	port := 6379

	args := os.Args[1:]
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

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", command)

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(response)
}
