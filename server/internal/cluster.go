package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	HashReplicas        = 150
	GossipInterval      = 1 * time.Second
	GossipTimeout       = 5 * time.Second
	HeartbeatInterval   = 1 * time.Second
	HealthCheckInterval = 2 * time.Second
	FailoverTimeout     = 10 * time.Second
)

type Cluster struct {
	mu                sync.RWMutex
	nodes             map[string]*NodeInfo
	hashRing          *ConsistentHash
	nodeID            string
	selfAddr          string
	gossipAddr        string
	gossipTicker      *time.Ticker
	shutdownChan      chan struct{}
	replication       *ReplicationState
	healthCheckTicker *time.Ticker
	failoverTicker    *time.Ticker
}

func NewCluster(nodeID, selfAddr, gossipAddr string) *Cluster {
	return &Cluster{
		nodes:             make(map[string]*NodeInfo),
		hashRing:          NewConsistentHash(HashReplicas),
		nodeID:            nodeID,
		selfAddr:          selfAddr,
		gossipAddr:        gossipAddr,
		gossipTicker:      time.NewTicker(GossipInterval),
		shutdownChan:      make(chan struct{}),
		replication:       NewReplicationState(),
		healthCheckTicker: time.NewTicker(HealthCheckInterval),
		failoverTicker:    time.NewTicker(FailoverTimeout),
	}
}

func (c *Cluster) AddNode(nodeID, addr string, role NodeRole) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodes[nodeID] = &NodeInfo{
		ID:        nodeID,
		Addr:      addr,
		Timestamp: time.Now().Unix(),
		Role:      role,
		Health:    true,
	}
	c.hashRing.AddNode(nodeID, addr)
	Info("Cluster: Added node %s at %s as %s", nodeID, addr, role.String())
}

func (c *Cluster) RemoveNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.nodes[nodeID]; ok {
		c.hashRing.RemoveNode(nodeID, node.Addr)
		delete(c.nodes, nodeID)
		Info("Cluster: Removed node %s", nodeID)
	}
}

func (c *Cluster) GetNodeForKey(key string) string {
	return c.hashRing.GetNode(key)
}

func (c *Cluster) GetMembers() []*NodeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	members := make([]*NodeInfo, 0, len(c.nodes))
	for _, node := range c.nodes {
		members = append(members, node)
	}
	return members
}

func (c *Cluster) GetNodeAddr(nodeID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if node, ok := c.nodes[nodeID]; ok {
		return node.Addr
	}
	return ""
}

func (c *Cluster) GetNodeID() string {
	return c.nodeID
}

func (c *Cluster) GetReplication() *ReplicationState {
	return c.replication
}

func (c *Cluster) SetNodeHealth(nodeID string, healthy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.nodes[nodeID]; ok {
		node.Health = healthy
	}
}

func (c *Cluster) IsNodeHealthy(nodeID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if node, ok := c.nodes[nodeID]; ok {
		return node.Health
	}
	return false
}

func (c *Cluster) StartGossip(peers []string) {
	if len(peers) == 0 {
		Info("Cluster: No peers to gossip with")
		return
	}

	for _, peer := range peers {
		go c.gossipWithNode(peer)
	}
}

func (c *Cluster) gossipWithNode(peerAddr string) {
	for {
		select {
		case <-c.shutdownChan:
			return
		case <-c.gossipTicker.C:
			c.sendGossip(peerAddr)
		}
	}
}

func (c *Cluster) sendGossip(peerAddr string) {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		Debug("Cluster: Failed to connect to peer %s: %v", peerAddr, err)
		return
	}
	defer conn.Close()

	c.mu.RLock()
	nodesJSON, _ := json.Marshal(c.nodes)
	replRole := c.replication.GetRole()
	c.mu.RUnlock()

	msg := fmt.Sprintf("CLUSTER_GOSSIP %s %s %s\n", c.nodeID, replRole.String(), nodesJSON)
	conn.Write([]byte(msg))

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		Debug("Cluster: Failed to read gossip response from %s: %v", peerAddr, err)
		return
	}

	if strings.HasPrefix(response, "+CLUSTER_GOSSIP") {
		parts := strings.Fields(response)
		if len(parts) >= 4 {
			var peerNodes map[string]*NodeInfo
			if err := json.Unmarshal([]byte(parts[3]), &peerNodes); err == nil {
				c.mergeNodes(peerNodes)
			}
		}
	}
}

func (c *Cluster) mergeNodes(peerNodes map[string]*NodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for nodeID, node := range peerNodes {
		if existing, ok := c.nodes[nodeID]; !ok || node.Timestamp > existing.Timestamp {
			c.nodes[nodeID] = node
			c.hashRing.AddNode(nodeID, node.Addr)
		}
	}
}

func (c *Cluster) StartHealthCheck() {
	go func() {
		for {
			select {
			case <-c.shutdownChan:
				return
			case <-c.healthCheckTicker.C:
				c.checkNodeHealth()
			}
		}
	}()
}

func (c *Cluster) checkNodeHealth() {
	c.mu.RLock()
	nodes := make([]*NodeInfo, 0, len(c.nodes))
	for _, n := range c.nodes {
		if n.ID != c.nodeID {
			nodes = append(nodes, n)
		}
	}
	c.mu.RUnlock()

	for _, node := range nodes {
		conn, err := net.DialTimeout("tcp", node.Addr, 2*time.Second)
		if err != nil {
			c.SetNodeHealth(node.ID, false)
			Warn("Cluster: Node %s (%s) is unhealthy: %v", node.ID, node.Addr, err)
			continue
		}
		conn.Write([]byte("PING\n"))
		reader := bufio.NewReader(conn)
		resp, _ := reader.ReadString('\n')
		conn.Close()

		if strings.Contains(resp, "PONG") {
			c.SetNodeHealth(node.ID, true)
		} else {
			c.SetNodeHealth(node.ID, false)
		}
	}
}

func (c *Cluster) StartFailoverMonitor(masterAddr string) {
	go func() {
		for {
			select {
			case <-c.shutdownChan:
				return
			case <-c.failoverTicker.C:
				if c.replication.GetRole() == RoleSlave {
					masterAddr := c.replication.GetMasterAddr()
					if masterAddr != "" && !c.isMasterReachable(masterAddr) {
						Info("Cluster: Master %s unreachable, initiating failover", masterAddr)
						c.initiateFailover()
					}
				}
			}
		}
	}()
}

func (c *Cluster) isMasterReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func (c *Cluster) initiateFailover() {
	Info("Cluster: Promoting self to master")
	c.replication.SetMaster("")
}

func (c *Cluster) SyncFromMaster(masterAddr string) error {
	conn, err := net.DialTimeout("tcp", masterAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "SYNC\n")

	reader := bufio.NewReader(conn)
	data, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read sync data: %w", err)
	}

	if strings.HasPrefix(data, "+") {
		var storeData map[string]string
		if err := json.Unmarshal([]byte(data[1:]), &storeData); err == nil {
			Info("Replication: Received %d keys from master", len(storeData))
		}
	}

	return nil
}

func (c *Cluster) ReplicateToSlaves(data map[string]string) {
	slaves := c.replication.GetSlaveAddrs()
	for _, slaveAddr := range slaves {
		go func(addr string) {
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				Warn("Replication: Failed to connect to slave %s: %v", addr, err)
				return
			}
			defer conn.Close()

			dataJSON, _ := json.Marshal(data)
			fmt.Fprintf(conn, "REPLICA_WRITE %s\n", dataJSON)
		}(slaveAddr)
	}
}

func (c *Cluster) Shutdown() {
	close(c.shutdownChan)
	c.gossipTicker.Stop()
	if c.healthCheckTicker != nil {
		c.healthCheckTicker.Stop()
	}
	if c.failoverTicker != nil {
		c.failoverTicker.Stop()
	}
}

func (c *Cluster) GetNodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.nodes)
}
