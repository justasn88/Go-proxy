package proxy

import (
	"awesomeProject11/internal/auth"
	"awesomeProject11/internal/domain"
	"awesomeProject11/internal/limits"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type Server struct {
	Repo domain.Repository
}

func (p *Server) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodConnect {
		p.HandleHTTPSConnect(w, r)
	} else {
		p.HandleHTTPRequests(w, r)
	}
}

func (p *Server) tunnelConn(dst io.WriteCloser, src io.ReadCloser, user domain.User, wg *sync.WaitGroup) {
	defer wg.Done()

	limiter := limits.NewTrackingWriter(user, dst)
	_, err := io.Copy(limiter, src)
	if err != nil {
		log.Printf("Tunnel copy error: %v", err)
	}
	dst.Close()
}

func (p *Server) getAuthorizedUser(w http.ResponseWriter, r *http.Request) (domain.User, string, func(), bool) {
	username, authorized := auth.Authenticate(r, p.Repo.GetCredentials())

	if !authorized {
		w.Header().Set("Proxy-Authorization", `Basic realm="Proxy"`)

		http.Error(w, "Authorization error", http.StatusProxyAuthRequired)
		return nil, "", nil, false
	}
	user := p.Repo.GetOrCreateUser(username)

	if user.IsOverDataLimit(limits.DataLimit) {
		http.Error(w, "Data limit has been reached", http.StatusTooManyRequests)
		return nil, "", nil, false
	}

	if !user.TryIncrementConnections(10) {
		http.Error(w, "Connection limits has been reached", http.StatusTooManyRequests)
		return nil, "", nil, false
	}

	cleanup := func() {
		user.DecrementConnections()
	}

	return user, username, cleanup, true

}

func (p *Server) HandleHTTPSConnect(w http.ResponseWriter, r *http.Request) {

	user, username, cleanup, ok := p.getAuthorizedUser(w, r)

	if !ok {
		return
	}
	defer cleanup()

	log.Printf("[HTTPS] User: %s | Server: %s", username, r.Host)

	targetConn, err := net.DialTimeout("tcp", r.Host, limits.TimeLimit)
	if err != nil {
		http.Error(w, "User exceeded time limit", http.StatusServiceUnavailable)
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

	err = clientConn.SetDeadline(deadline)
	if err != nil {
		log.Printf("Failed to set deadline: %v", err)
		return
	}

	err = targetConn.SetDeadline(deadline)
	if err != nil {
		log.Printf("Failed to set deadline: %v", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go p.tunnelConn(targetConn, clientConn, user, &wg)
	go p.tunnelConn(clientConn, targetConn, user, &wg)

	wg.Wait()
}

func (p *Server) HandleHTTPRequests(w http.ResponseWriter, r *http.Request) {
	user, username, cleanup, ok := p.getAuthorizedUser(w, r)

	if !ok {
		return
	}
	defer cleanup()

	log.Printf("[HTTP] User: %s | Server: %s", username, r.Host)

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
			http.Error(w, "User exceeded time limit", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Could not reach server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	tracker := limits.NewTrackingWriter(user, &limits.NopCloserWriter{ResponseWriter: w})
	if _, err = io.Copy(tracker, resp.Body); err != nil {
		log.Printf("Connection error: %v", err)
	}
}
