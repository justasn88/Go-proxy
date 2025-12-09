package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"time"
)

const DataLimit = 1024 * 1024 * 1024
const TimeLimit = 1 * time.Hour

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

type dataTrackingWriter struct {
	user *UserState
	wc   io.WriteCloser
}

type nopCloserWriter struct {
	http.ResponseWriter
}

func (n *nopCloserWriter) Close() error { return nil }

const staticAuth = "user:pass"

var globalState GlobalState

func (d *dataTrackingWriter) Write(p []byte) (int, error) {
	d.user.mu.Lock()
	defer d.user.mu.Unlock()

	if d.user.DataUsed+int64(len(p)) > DataLimit || d.user.DataUsed >= DataLimit {
		d.wc.Close()
		return 0, fmt.Errorf("Duomenu limitas viršytas")
	}

	d.user.DataUsed += int64(len(p))

	return d.wc.Write(p)
}

func authenticate(r *http.Request) (username string, authorized bool) {
	authHeader := r.Header.Get("Proxy-Authorization")
	if authHeader == "" {
		return "", false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Basic" {
		return "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	credentials := string(decoded)
	credParts := strings.Split(credentials, ":")

	user := credParts[0]
	pass := credParts[1]

	if allowedPass, ok := globalState.ValidCredentials[user]; ok && allowedPass == pass {
		return user, true
	}
	return "", false
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodConnect {
		HandleHTTPSConnect(w, r)
	} else {
		handleHTTPRequests(w, r)
	}
}

func tunnelConn(dst io.WriteCloser, src io.ReadCloser, user *UserState) {
	defer dst.Close()
	defer src.Close()
	limiter := &dataTrackingWriter{
		user: user,
		wc:   dst,
	}
	io.Copy(limiter, src)

}

func HandleHTTPSConnect(w http.ResponseWriter, r *http.Request) {
	username, authorized := authenticate(r)

	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}
	globalState.mu.Lock()
	user, exists := globalState.UserMap[username]
	if !exists {
		user = &UserState{}
		globalState.UserMap[username] = user
	}
	globalState.mu.Unlock()

	user.mu.Lock()

	if globalState.UserMap[username].ActiveConnections >= 10 {
		http.Error(w, "Virsytas maksimalus leistinas prisijungimų skaičius", http.StatusTooManyRequests)
		return
	}
	user.ActiveConnections++
	user.mu.Unlock()

	defer func() {
		user.mu.Lock()
		user.ActiveConnections--
		user.mu.Unlock()
	}()

	log.Printf("[HTTPS] Vartotojas: %s | Serveris: %s", username, r.Host)

	targetConn, err := net.DialTimeout("tcp", r.Host, TimeLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		targetConn.Close()
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		targetConn.Close()
		return
	}

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		clientConn.Close()
		targetConn.Close()
		return
	}

	deadline := time.Now().Add(TimeLimit)
	clientConn.SetDeadline(deadline)
	targetConn.SetDeadline(deadline)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		tunnelConn(targetConn, clientConn, user)
	}()
	go func() {
		defer wg.Done()
		tunnelConn(clientConn, targetConn, user)
	}()
	wg.Wait()
}

func handleHTTPRequests(w http.ResponseWriter, r *http.Request) {
	username, authorized := authenticate(r)
	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}

	log.Printf("[HTTP] Vartotojas: %s | Serveris: %s", username, r.Host)

	globalState.mu.Lock()
	user, exists := globalState.UserMap[username]
	if !exists {
		user = &UserState{}
		globalState.UserMap[username] = user
	}
	globalState.mu.Unlock()

	user.mu.Lock()
	if user.ActiveConnections >= 10 {
		user.mu.Unlock()
		http.Error(w, "Limitas virsytas", http.StatusTooManyRequests)
		return
	}
	user.ActiveConnections++
	user.mu.Unlock()

	defer func() {
		user.mu.Lock()
		user.ActiveConnections--
		user.mu.Unlock()
	}()

	user.mu.Lock()
	if user.DataUsed > DataLimit {
		user.mu.Unlock()
		http.Error(w, "Duomenu limitas viršytas", http.StatusTooManyRequests)
		return
	}
	user.mu.Unlock()

	req := r.Clone(r.Context())
	req.Header.Del("Proxy-Authorization")
	req.RequestURI = ""

	client := &http.Client{Timeout: 1 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Nepavyko pasiekti serverio", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	tracker := &dataTrackingWriter{user: user, wc: &nopCloserWriter{w}}
	n, err := io.Copy(tracker, resp.Body)
	if err != nil {
		log.Printf("Ryšys nutrauktas: %v", err)
	}
	log.Printf("Užklausa baigta, persiųsta %d baitų", n)
}

func main() {
	log.SetOutput(os.Stdout)

	allowedUser := map[string]string{"user": "pass"}
	globalState = GlobalState{
		UserMap:          map[string]*UserState{},
		ValidCredentials: allowedUser,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(proxyHandler)))
}
