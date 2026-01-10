package repo

import (
	"awesomeProject11/internal/domain"
	"sync"
)

type memoryUser struct {
	mu        sync.Mutex
	dataUsed  int64
	activeCon int64
}

type InMemoryRepo struct {
	mu          sync.Mutex
	users       map[string]*memoryUser
	credentials map[string]string
}

func (u *memoryUser) AddData(n int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dataUsed += n
}

func (u *memoryUser) IsOverDataLimit(limit int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.dataUsed >= limit
}

func (u *memoryUser) TryIncrementConnections(max int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.activeCon >= max {
		return false
	}
	u.activeCon++
	return true
}
func (u *memoryUser) DecrementConnections() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.activeCon--
}

func NewMemoryRepo(creds map[string]string) domain.Repository {
	return &InMemoryRepo{
		users:       make(map[string]*memoryUser),
		credentials: creds,
	}
}

func (r *InMemoryRepo) GetOrCreateUser(username string) domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()

	if u, ok := r.users[username]; ok {
		return u
	}
	newUser := &memoryUser{}
	r.users[username] = newUser
	return newUser
}

func (r *InMemoryRepo) GetCredentials() map[string]string {
	return r.credentials
}
