package proxy

import (
	"awesomeProject11/auth"
	"awesomeProject11/limits"
	"awesomeProject11/state"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type ProxyServer struct {
	GlobalState *state.GlobalState
}

func (p *ProxyServer) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodConnect {
		p.HandleHTTPSConnect(w, r)
	} else {
		p.HandleHTTPRequests(w, r)
	}
}

func (p *ProxyServer) tunnelConn(dst io.WriteCloser, src io.ReadCloser, user *state.UserState) {
	defer dst.Close()
	defer src.Close()
	limiter := limits.NewTrackingWriter(user, dst)
	io.Copy(limiter, src)

}

func (p *ProxyServer) HandleHTTPSConnect(w http.ResponseWriter, r *http.Request) {
	username, authorized := auth.Authenticate(r, p.GlobalState.ValidCredentials)

	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}
	p.GlobalState.Lock()
	user, exists := p.GlobalState.UserMap[username]
	if !exists {
		user = &state.UserState{}
		p.GlobalState.UserMap[username] = user
	}
	p.GlobalState.Unlock()

	user.Lock()

	if p.GlobalState.UserMap[username].ActiveConnections >= 10 {
		user.Unlock()
		http.Error(w, "Virsytas maksimalus leistinas prisijungimų skaičius", http.StatusTooManyRequests)
		return
	}
	user.ActiveConnections++
	user.Unlock()

	defer func() {
		user.Lock()
		user.ActiveConnections--
		user.Unlock()
	}()
	user.Lock()
	if user.DataUsed > limits.DataLimit {
		user.Unlock()
		http.Error(w, "Duomenu limitas virsytas", http.StatusForbidden)
		return
	}
	user.Unlock()

	log.Printf("[HTTPS] Vartotojas: %s | Serveris: %s", username, r.Host)

	targetConn, err := net.DialTimeout("tcp", r.Host, limits.TimeLimit)
	if err != nil {
		http.Error(w, "Vartotojas virsijo uzklausos laiko limita", http.StatusServiceUnavailable)
		return
	}
	defer targetConn.Close()

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
	defer clientConn.Close()

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		clientConn.Close()
		targetConn.Close()
		return
	}

	deadline := time.Now().Add(limits.TimeLimit)
	clientConn.SetDeadline(deadline)
	targetConn.SetDeadline(deadline)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		p.tunnelConn(targetConn, clientConn, user)
	}()
	go func() {
		defer wg.Done()
		p.tunnelConn(clientConn, targetConn, user)
	}()
	wg.Wait()
}

func (p *ProxyServer) HandleHTTPRequests(w http.ResponseWriter, r *http.Request) {
	username, authorized := auth.Authenticate(r, p.GlobalState.ValidCredentials)
	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}

	log.Printf("[HTTP] Vartotojas: %s | Serveris: %s", username, r.Host)

	p.GlobalState.Lock()
	user, exists := p.GlobalState.UserMap[username]
	if !exists {
		user = &state.UserState{}
		p.GlobalState.UserMap[username] = user
	}
	p.GlobalState.Unlock()

	user.Lock()
	if user.ActiveConnections >= 10 {
		user.Unlock()
		http.Error(w, "Limitas virsytas", http.StatusTooManyRequests)
		return
	}
	user.ActiveConnections++
	user.Unlock()

	defer func() {
		user.Lock()
		user.ActiveConnections--
		user.Unlock()
	}()

	user.Lock()
	if user.DataUsed > limits.DataLimit {
		user.Unlock()
		http.Error(w, "Duomenu limitas viršytas", http.StatusTooManyRequests)
		return
	}
	user.Unlock()

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

	if req.Body != nil {
		trackedBody := limits.NewTrackingReader(user, req.Body)
		req.Body = trackedBody
	}

	tracker := limits.NewTrackingWriter(user, &limits.NopCloserWriter{w})
	_, err = io.Copy(tracker, resp.Body)
	if err != nil {
		log.Printf("Ryšys nutrauktas: %v", err)
	}
}
