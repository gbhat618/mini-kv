package internal

import (
	"sync"
	"time"
)

type ReplicationState struct {
	mu            sync.RWMutex
	role          NodeRole
	masterAddr    string
	slaveAddrs    []string
	replOffset    int64
	lastSyncTime  time.Time
	isReplicating bool
}

func NewReplicationState() *ReplicationState {
	return &ReplicationState{
		role:       RoleMaster,
		masterAddr: "",
		slaveAddrs: make([]string, 0),
		replOffset: 0,
	}
}

func (rs *ReplicationState) GetRole() NodeRole {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.role
}

func (rs *ReplicationState) SetMaster(addr string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if addr == "" {
		rs.role = RoleMaster
		rs.masterAddr = ""
	} else {
		rs.role = RoleSlave
		rs.masterAddr = addr
	}
	Info("Replication: Set role to %s, master: %s", rs.role.String(), addr)
}

func (rs *ReplicationState) AddSlave(addr string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, s := range rs.slaveAddrs {
		if s == addr {
			return
		}
	}
	rs.slaveAddrs = append(rs.slaveAddrs, addr)
	Info("Replication: Added slave %s", addr)
}

func (rs *ReplicationState) RemoveSlave(addr string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for i, s := range rs.slaveAddrs {
		if s == addr {
			rs.slaveAddrs = append(rs.slaveAddrs[:i], rs.slaveAddrs[i+1:]...)
			Info("Replication: Removed slave %s", addr)
			break
		}
	}
}

func (rs *ReplicationState) GetSlaveAddrs() []string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.slaveAddrs
}

func (rs *ReplicationState) GetMasterAddr() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.masterAddr
}

func (rs *ReplicationState) IsReplicating() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.isReplicating
}

func (rs *ReplicationState) SetReplicating(replicating bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.isReplicating = replicating
}

func (rs *ReplicationState) GetReplOffset() int64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.replOffset
}

func (rs *ReplicationState) IncrementReplOffset() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.replOffset++
}
