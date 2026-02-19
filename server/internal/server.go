package internal

import (
	"bufio"
	"crypto/tls"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type Server struct {
	addr             string
	kv               *MiniKV
	tlsConfig        *tls.Config
	connCount        atomic.Int32
	shutdownChan     chan struct{}
	cluster          *Cluster
	commandProcessor *CommandProcessor
}

func NewServer(addr string, gossipAddr string) *Server {
	var cluster *Cluster
	if ClusterMode {
		cluster = NewCluster(NodeID, addr, gossipAddr)
		cluster.AddNode(NodeID, addr, RoleMaster)
	}

	s := &Server{
		addr:         addr,
		kv:           NewMiniKV(),
		shutdownChan: make(chan struct{}),
		cluster:      cluster,
	}
	s.commandProcessor = NewCommandProcessor(s)
	return s
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
		Info("Mini-KV Server started on %s with TLS (log level: %s)", s.addr, strings.ToUpper(GetLogPrefix()))
	} else {
		ln, err = net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
		Info("Mini-KV Server started on %s (log level: %s)", s.addr, strings.ToUpper(GetLogPrefix()))
	}

	if AuthPassword != "" {
		Info("Authentication enabled")
	}
	if s.tlsConfig != nil {
		Info("TLS enabled")
	}
	Info("Max connections: %d, Max key size: %d, Max value size: %d", MaxConns, MaxKeySize, MaxValueSize)

	if s.cluster != nil {
		Info("Cluster mode enabled")
		if s.cluster.replication != nil {
			Info("Replication: Initial role is %s", s.cluster.replication.GetRole().String())
		}

		if ClusterPeers != "" {
			peers := strings.Split(ClusterPeers, ",")
			for i := range peers {
				peers[i] = strings.TrimSpace(peers[i])
			}
			Info("Cluster: Connecting to peers: %v", peers)
			s.cluster.StartGossip(peers)
		}

		s.cluster.StartHealthCheck()
		s.cluster.StartFailoverMonitor("")
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		Error("Shutdown signal received")
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
				Error("Accept error: %v", err)
				continue
			}
		}

		if s.connCount.Load() >= int32(MaxConns) {
			Error("Connection limit reached, rejecting connection from %s", conn.RemoteAddr())
			conn.Close()
			continue
		}

		s.connCount.Add(1)
		Debug("Accepted connection from %s (active: %d)", conn.RemoteAddr(), s.connCount.Load())
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		s.connCount.Add(-1)
		conn.Close()
		Debug("Connection closed from %s", conn.RemoteAddr())
	}()

	if ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Duration(ReadTimeout) * time.Second))
	}

	reader := bufio.NewReader(conn)
	txn := NewTransaction()
	inTxn := false
	authenticated := AuthPassword == ""

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		Debug("Received command: %q", line)

		if !authenticated {
			response := s.processAuth(line)
			conn.Write([]byte(response))
			continue
		}

		response := s.commandProcessor.ProcessCommand(line, txn, &inTxn)
		conn.Write([]byte(response))

		if ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(time.Duration(ReadTimeout) * time.Second))
		}
	}
}

func (s *Server) processAuth(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 || strings.ToUpper(parts[0]) != "AUTH" {
		return "-ERR authentication required\r\n"
	}

	inputPassword := parts[1]
	if AuthPassword != "" && strings.Compare(AuthPassword, inputPassword) == 0 {
		return "+OK\r\n"
	}

	Error("Failed authentication attempt")
	return "-ERR invalid password\r\n"
}

func (s *Server) GetAddr() string {
	return s.addr
}

func (s *Server) GetCluster() *Cluster {
	return s.cluster
}

func (s *Server) GetKV() *MiniKV {
	return s.kv
}

func (s *Server) ProcessCommand(cmd string, txn *Transaction, inTxn *bool) string {
	return s.commandProcessor.ProcessCommand(cmd, txn, inTxn)
}

func (s *Server) GetConnectionCount() int32 {
	return s.connCount.Load()
}
