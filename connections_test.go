package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const connections = 12

func TestHTTPConnections(t *testing.T) {
	globalState.mu.Lock()

	globalState.UserMap = make(map[string]*UserState)
	globalState.ValidCredentials = map[string]string{"user": "pass"}

	globalState.UserMap["user"] = &UserState{}

	globalState.mu.Unlock()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	targetURL := targetServer.URL

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = strings.TrimPrefix(targetURL, "http://")
		r.URL.Scheme = "http"
		handleHTTPRequests(w, r)
	})
	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	var user string
	var pass string

	for key, value := range globalState.ValidCredentials {
		user = key
		pass = value
	}

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))

	var wg sync.WaitGroup
	statusCodes := make(chan int, connections)

	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			transport := &http.Transport{
				DisableKeepAlives: true,
				MaxIdleConns:      0,
			}
			client := &http.Client{
				Timeout:   40 * time.Second,
				Transport: transport,
			}
			req, _ := http.NewRequest(http.MethodGet, proxyServer.URL, nil)
			req.Header.Set("Proxy-Authorization", authHeader)
			resp, err := client.Do(req)

			if err != nil {
				t.Logf("Uzklausa nepavyko: %v", err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
			statusCodes <- resp.StatusCode

		}()
	}
	wg.Wait()
	close(statusCodes)

	var successCount int
	var rejectedCount int

	for status := range statusCodes {
		if status == http.StatusOK {
			successCount++
		} else if status == http.StatusTooManyRequests {
			rejectedCount++
		} else {
			t.Errorf("netiketa klaida: %v", status)
		}
	}

	t.Errorf("tiketasi 10 sekmingu uzklausu, gauta sekmingu uzklausu: %v, gauta nesekmingu uzklausu: %v", successCount, rejectedCount)

}
