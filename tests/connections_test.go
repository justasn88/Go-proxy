package tests

import (
	"awesomeProject11/internal/proxy"
	"awesomeProject11/internal/repo"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const connections = 14

func TestHTTPConnections(t *testing.T) {

	creds := map[string]string{"user": "pass"}
	repository := repo.NewMemoryRepo(creds)

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
			defer resp.Body.Close()
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
		if status == http.StatusOK {
			successCount++
		} else if status == http.StatusTooManyRequests {
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
