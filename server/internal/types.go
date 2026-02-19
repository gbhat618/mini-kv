package internal

import "sync"

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type NodeRole int

const (
	RoleUnknown NodeRole = iota
	RoleMaster
	RoleSlave
)

func (r NodeRole) String() string {
	switch r {
	case RoleMaster:
		return "master"
	case RoleSlave:
		return "slave"
	default:
		return "unknown"
	}
}

type NodeInfo struct {
	ID        string
	Addr      string
	Timestamp int64
	Role      NodeRole
	Health    bool
}

type Transaction struct {
	pending map[string]string
	deleted map[string]bool
	mu      sync.Mutex
}

func NewTransaction() *Transaction {
	return &Transaction{
		pending: make(map[string]string),
		deleted: make(map[string]bool),
	}
}
