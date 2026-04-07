package proxy

import (
	"awesomeProject11/internal/auth"
	"awesomeProject11/internal/domain"
	"awesomeProject11/internal/limits"
	"errors"
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

func (s *Server) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodConnect {
		s.HandleHTTPSRequests(w, r)
	} else {
		s.HandleHTTPRequests(w, r)
	}
}

func (s *Server) tunnelConn(dst io.WriteCloser, src io.ReadCloser, user domain.User, wg *sync.WaitGroup) {
	defer wg.Done()

	defer func() {
		if closeError := dst.Close(); closeError != nil {
			log.Printf("Tunnel close error: %v", closeError)
		}
	}()

	defer func() {
		if closeError := src.Close(); closeError != nil {
			log.Printf("Tunnel close error: %v", closeError)
		}
	}()

	limiter := limits.NewTrackingWriter(user, dst)
	_, err := io.Copy(limiter, src)
	if err != nil {
		log.Printf("Tunnel copy error: %v", err)
	}

}

func (s *Server) authenticateUser(w http.ResponseWriter, r *http.Request) (domain.User, string, func(), bool) {
	username, password, found := auth.ExtractCredentials(r)

	if !found || !s.Repo.ValidateUser(username, password) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="Proxy"`)

		http.Error(w, "Authentication error", http.StatusProxyAuthRequired)
		return nil, "", nil, false
	}
	user := s.Repo.GetOrCreateUser(username)
	dataLimit, maxConnections := s.Repo.GetUserLimits(username)

	if user.IsOverDataLimit(dataLimit) {
		http.Error(w, "Data limit has been reached", http.StatusTooManyRequests)
		return nil, "", nil, false
	}
	if !user.TryIncrementConnections(maxConnections) {
		http.Error(w, "Connection limits has been reached", http.StatusTooManyRequests)
		return nil, "", nil, false
	}
	cleanup := func() {
		user.DecrementConnections()
	}
	return user, username, cleanup, true
}

func (s *Server) HandleHTTPSRequests(w http.ResponseWriter, r *http.Request) {

	user, username, cleanup, ok := s.authenticateUser(w, r)

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
	defer func() {
		err := targetConn.Close()

		if err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("Target close error: %v", err)
		}
	}()

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
	defer func() {
		err := clientConn.Close()

		if err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("Client close error: %v", err)
		}
	}()

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

	go s.tunnelConn(targetConn, clientConn, user, &wg)
	go s.tunnelConn(clientConn, targetConn, user, &wg)

	wg.Wait()
}

func (s *Server) HandleHTTPRequests(w http.ResponseWriter, r *http.Request) {
	user, username, cleanup, ok := s.authenticateUser(w, r)

	if !ok {
		return
	}
	defer cleanup()

	log.Printf("[HTTP] User: %s | Server: %s", username, r.Host)

	req, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	for k, v := range r.Header {
		req.Header[k] = v
	}

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = r.Host
	}
	req.Header.Del("Proxy-Authorization")

	if req.Body != nil {
		req.Body = limits.NewTrackingReader(user, req.Body)
	}

	client := &http.Client{
		Timeout: limits.TimeLimit,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Proxy transport error: %v", err)
		if os.IsTimeout(err) {
			http.Error(w, "User exceeded time limit", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Could not reach server: "+err.Error(), http.StatusBadGateway)
		return
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Response body close error: %v", err)
		}
	}()

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
