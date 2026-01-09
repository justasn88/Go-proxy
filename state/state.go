package state

import (
	"sync"
)

type UserState struct {
	mu                sync.Mutex
	activeConnections int64
	dataUsed          int64
}
type GlobalState struct {
	mu               sync.Mutex
	userMap          map[string]*UserState
	validCredentials map[string]string
}

func (u *UserState) DecrementConnections() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.activeConnections--
}

func (u *UserState) TryIncrementConnections(max int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.activeConnections >= max {
		return false
	}
	u.activeConnections++
	return true
}

func (u *UserState) IsOverLimit(limit int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.dataUsed > limit
}

func (u *UserState) AddData(n int64) {
	u.Lock()
	u.dataUsed += n
	u.Unlock()
}

func (g *GlobalState) GetOrCreateUser(username string) *UserState {
	g.Lock()
	defer g.Unlock()
	user, exists := g.userMap[username]
	if !exists {
		user = &UserState{}
		g.userMap[username] = user
	}
	return user
}

func (g *GlobalState) GetCredentials() map[string]string {
	return g.validCredentials
}

func NewGlobalState(credentials map[string]string) *GlobalState {
	return &GlobalState{
		userMap:          map[string]*UserState{},
		validCredentials: credentials,
	}
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
