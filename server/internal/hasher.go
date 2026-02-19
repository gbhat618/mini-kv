package internal

import (
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
)

type ConsistentHash struct {
	mu         sync.RWMutex
	replicas   int
	ring       map[uint32]string
	sortedKeys []uint32
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas:   replicas,
		ring:       make(map[uint32]string),
		sortedKeys: make([]uint32, 0),
	}
}

func (ch *ConsistentHash) AddNode(nodeID, addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := fmt.Sprintf("%s-%d-%s", nodeID, i, addr)
		hash := crc32.ChecksumIEEE([]byte(key))
		ch.ring[hash] = nodeID
		ch.sortedKeys = append(ch.sortedKeys, hash)
	}

	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

func (ch *ConsistentHash) RemoveNode(nodeID string, addr string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := fmt.Sprintf("%s-%d-%s", nodeID, i, addr)
		hash := crc32.ChecksumIEEE([]byte(key))
		delete(ch.ring, hash)
	}

	ch.sortedKeys = make([]uint32, 0)
	for hash := range ch.ring {
		ch.sortedKeys = append(ch.sortedKeys, hash)
	}

	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	hash := crc32.ChecksumIEEE([]byte(key))
	idx := 0
	for i, k := range ch.sortedKeys {
		if hash <= k {
			idx = i
			break
		}
		if i == len(ch.sortedKeys)-1 {
			idx = 0
			break
		}
	}

	return ch.ring[ch.sortedKeys[idx]]
}

func (ch *ConsistentHash) GetAllNodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	nodes := make(map[string]bool)
	for _, nodeID := range ch.ring {
		nodes[nodeID] = true
	}

	result := make([]string, 0, len(nodes))
	for nodeID := range nodes {
		result = append(result, nodeID)
	}
	return result
}

func (ch *ConsistentHash) Size() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.ring)
}
