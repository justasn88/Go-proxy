package main

import (
	"awesomeProject11/proxy"
	"awesomeProject11/state"
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
	state.GlobalStateInstance.Lock()

	state.GlobalStateInstance.UserMap = make(map[string]*state.UserState)
	state.GlobalStateInstance.ValidCredentials = map[string]string{"user": "pass"}

	state.GlobalStateInstance.UserMap["user"] = &state.UserState{}

	state.GlobalStateInstance.Unlock()

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	targetURL := targetServer.URL

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = strings.TrimPrefix(targetURL, "http://")
		r.URL.Scheme = "http"
		proxy.HandleHTTPRequests(w, r)
	})
	proxyServer := httptest.NewServer(proxyHandler)
	defer proxyServer.Close()

	var user string
	var pass string

	for key, value := range state.GlobalStateInstance.ValidCredentials {
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

	expectedSuccess := 10
	expectedRejected := connections - expectedSuccess

	if successCount != expectedSuccess || rejectedCount != expectedRejected {
		t.Errorf("Limitu tikrinimo klaida. Tikėtasi 10 OK ir %d TooManyRequests. Gauta: %d OK, %d Rejected",
			expectedRejected, successCount, rejectedCount)
	} else {
		t.Logf("Limitu testas sėkmingas. Gauta %d OK ir %d Rejected", successCount, rejectedCount)
	}
}
