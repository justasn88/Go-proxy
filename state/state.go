package state

import (
	"sync"
)

type UserState struct {
	mu                sync.Mutex
	ActiveConnections int
	DataUsed          int64
}
type GlobalState struct {
	mu               sync.Mutex
	UserMap          map[string]*UserState
	ValidCredentials map[string]string
}

func (u *UserState) Lock() {
	u.mu.Lock()
}

func (u *UserState) Unlock() {
	u.mu.Unlock()
}

func (g *GlobalState) Lock() {
	g.mu.Lock()
}

func (g *GlobalState) Unlock() {
	g.mu.Unlock()
}
