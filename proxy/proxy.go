package proxy

import (
	"awesomeProject11/auth"
	"awesomeProject11/limits"
	"awesomeProject11/state"
	"io"
	"log"
	"net"
	"net/http"
	"os"
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

func (p *ProxyServer) tunnelConn(dst io.WriteCloser, src io.ReadCloser, user *state.UserState, wg *sync.WaitGroup) {
	defer wg.Done()

	limiter := limits.NewTrackingWriter(user, dst)
	io.Copy(limiter, src)

	dst.Close()
}

func (p *ProxyServer) getAuthorizedUser(w http.ResponseWriter, r *http.Request) (*state.UserState, string, func(), bool) {
	username, authorized := auth.Authenticate(r, p.GlobalState.ValidCredentials)

	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return nil, "", nil, false
	}
	p.GlobalState.Lock()
	user, exists := p.GlobalState.UserMap[username]
	if !exists {
		user = &state.UserState{}
		p.GlobalState.UserMap[username] = user
	}
	p.GlobalState.Unlock()

	user.Lock()
	defer user.Unlock()

	if user.DataUsed > limits.DataLimit {
		http.Error(w, "Duomenu limitas viršytas", http.StatusTooManyRequests)
		return nil, "", nil, false
	}

	if user.ActiveConnections >= 10 {
		http.Error(w, "Virsytas maksimalus leistinas prisijungimu skaicius", http.StatusTooManyRequests)
		return nil, "", nil, false
	}
	user.ActiveConnections++

	cleanup := func() {
		user.Lock()
		user.ActiveConnections--
		user.Unlock()
	}

	return user, username, cleanup, true

}

func (p *ProxyServer) HandleHTTPSConnect(w http.ResponseWriter, r *http.Request) {

	user, username, cleanup, ok := p.getAuthorizedUser(w, r)

	if !ok {
		return
	}
	defer cleanup()

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
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	if _, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	deadline := time.Now().Add(limits.TimeLimit)
	clientConn.SetDeadline(deadline)
	targetConn.SetDeadline(deadline)

	var wg sync.WaitGroup
	wg.Add(2)

	go p.tunnelConn(targetConn, clientConn, user, &wg)
	go p.tunnelConn(clientConn, targetConn, user, &wg)

	wg.Wait()
}

func (p *ProxyServer) HandleHTTPRequests(w http.ResponseWriter, r *http.Request) {
	user, username, cleanup, ok := p.getAuthorizedUser(w, r)

	if !ok {
		return
	}
	defer cleanup()

	log.Printf("[HTTP] Vartotojas: %s | Serveris: %s", username, r.Host)

	req := r.Clone(r.Context())
	req.Header.Del("Proxy-Authorization")
	req.RequestURI = ""

	if req.Body != nil {
		req.Body = limits.NewTrackingReader(user, req.Body)
	}

	client := &http.Client{Timeout: limits.TimeLimit}
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			http.Error(w, "Vartotojas virsijo uzklausos laiko limita", http.StatusServiceUnavailable)
			return
		}
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

	tracker := limits.NewTrackingWriter(user, &limits.NopCloserWriter{w})
	if _, err = io.Copy(tracker, resp.Body); err != nil {
		log.Printf("Ryšys nutrauktas: %v", err)
	}
}
