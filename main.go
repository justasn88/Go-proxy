package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"time"
)

const DataLimit = 1024 * 1024 * 1024
const TimeLimit = 1 * time.Hour

type UserState struct {
	ActiveConnections int
	DataUsed          int64
}
type GlobalState struct {
	UserMap          map[string]*UserState
	ValidCredentials map[string]string
}

const staticAuth = "user:pass"

var globalstate GlobalState

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

	if allowedPass, ok := globalstate.ValidCredentials[user]; ok && allowedPass == pass {
		return user, true
	}
	return "", false
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Println(authenticate(req))
	fmt.Fprintf(w, "pong\n")
	response, err := http.Get("localhost:8080")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(response)
}

func incrementConnections(username string) {
	globalstate.UserMap[username].ActiveConnections++

}

func decreaseConnetions(username string) {
	globalstate.UserMap[username].ActiveConnections--
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodConnect {
		HandleHTTPSConnect(w, r)
	} else {
		handleHTTPRequests(w, r)
	}
}

func tunnelConn(dst io.WriteCloser, src io.ReadCloser) int64 {
	bytesCopied, _ := io.Copy(dst, src)
	dst.Close()
	src.Close()
	return bytesCopied
}

func HandleHTTPSConnect(w http.ResponseWriter, r *http.Request) {
	username, authorized := authenticate(r)

	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}

	incrementConnections(username)

	defer decreaseConnetions(username)

	if globalstate.UserMap[username].ActiveConnections > 10 {
		http.Error(w, "Virsytas maksimalus leistinas prisijungimų skaičius", http.StatusTooManyRequests)
		return
	}

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

	log.Println("Tunnel established")

	go tunnelConn(targetConn, clientConn)
	go tunnelConn(clientConn, targetConn)
}

func checkAndUpdateDataUsed(username string, dataSize int64) error {

	if globalstate.UserMap[username].DataUsed+dataSize > DataLimit {
		return fmt.Errorf("duomenų limitas (1GB) bus viršytas. Liko: %d B", DataLimit-globalstate.UserMap[username].DataUsed)
	}

	globalstate.UserMap[username].DataUsed += dataSize
	return nil
}

func handleHTTPRequests(w http.ResponseWriter, r *http.Request) {

	username, authorized := authenticate(r)

	if !authorized {
		http.Error(w, "Autentifikavimo klaida", http.StatusUnauthorized)
		return
	}

	incrementConnections(username)

	defer decreaseConnetions(username)

	if globalstate.UserMap[username].ActiveConnections > 10 {
		http.Error(w, "Virsytas maksimalus leistinas prisijungimų skaičius", http.StatusTooManyRequests)
		return
	}

	requestBytes, _ := httputil.DumpRequest(r, false)
	headerSize := int64(len(requestBytes))
	contentLength := r.ContentLength
	if contentLength == -1 {
		contentLength = 0
	}
	incomingSize := headerSize + contentLength

	err := checkAndUpdateDataUsed(username, incomingSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	}

	req := r.Clone(r.Context())
	fmt.Println(req)
	req.Header.Del("Proxy-Authorization")
	req.RequestURI = ""

	client := http.Client{
		Timeout: 1 * time.Hour,
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Serverio klaida", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	fmt.Println(resp.Header)

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

}

func main() {
	log.SetOutput(os.Stdout)

	allowedUser := map[string]string{"user": "pass"}
	globalstate = GlobalState{
		UserMap:          map[string]*UserState{},
		ValidCredentials: allowedUser,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(proxyHandler)))
}

//var auth string
//
//for key, value := range globalState.ValidCredentials {
//	auth = fmt.Sprintf("%s:%s", key, value)
//}
//basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
//fmt.Println(basicAuth)
