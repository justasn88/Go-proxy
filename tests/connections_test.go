package tests

import (
	"awesomeProject11/internal/domain"
	"awesomeProject11/internal/proxy"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const connections = 14

type mockUser struct {
	activeConns int64
	mu          sync.Mutex
}

func (u *mockUser) AddData(n int64)                  {}
func (u *mockUser) IsOverDataLimit(limit int64) bool { return false }
func (u *mockUser) TryIncrementConnections(max int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.activeConns >= max {
		return false
	}
	u.activeConns++
	return true
}
func (u *mockUser) DecrementConnections() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.activeConns--
}

type mockRepo struct {
	users map[string]*mockUser
	mu    sync.Mutex
}

func (m *mockRepo) GetOrCreateUser(username string) domain.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.users == nil {
		m.users = make(map[string]*mockUser)
	}
	if _, exists := m.users[username]; !exists {
		m.users[username] = &mockUser{}
	}
	return m.users[username]
}

func (m *mockRepo) ValidateUser(username, password string) bool {
	return username == "user" && password == "pass"
}

func (m *mockRepo) GetUserLimits(username string) (int64, int64) {
	return 1073741824, 10
}

func TestHTTPConnections(t *testing.T) {

	repository := &mockRepo{}

	proxyInstance := &proxy.Server{
		Repo: repository,
	}

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	targetURL := targetServer.URL

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = strings.TrimPrefix(targetURL, "http://")
		r.URL.Scheme = "http"
		proxyInstance.HandleHTTPRequests(w, r)
	})

	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))

	var wg sync.WaitGroup
	statusCodes := make(chan int, connections)

	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := http.Client{
				Timeout:   40 * time.Second,
				Transport: &http.Transport{DisableKeepAlives: true},
			}

			req, _ := http.NewRequest(http.MethodGet, proxyServer.URL, nil)
			req.Header.Set("Proxy-Authorization", authHeader)
			resp, err := client.Do(req)

			if err != nil {
				t.Logf("Request failed: %v", err)
				return
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Response body close error: %v", err)
				}
			}()

			_, err = io.ReadAll(resp.Body)

			if err != nil {
				t.Logf("Failed to read body: %v", err)
			}

			statusCodes <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(statusCodes)

	var successCount int
	var rejectedCount int

	for status := range statusCodes {
		switch status {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			rejectedCount++
		}
	}

	expectedSuccess := 10
	expectedRejected := connections - expectedSuccess

	if successCount != expectedSuccess || rejectedCount != expectedRejected {
		t.Errorf("Connection limit test error. expected to get %v statusOK and %v TooManyRequests. Got: %v StatusOK and %v TooManyRequests", expectedSuccess, expectedRejected, successCount, rejectedCount)
	} else {
		t.Logf("Test passed. Got %v StatusOK and %v TooManyRequests", successCount, rejectedCount)
	}
}
