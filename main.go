package main

import (
	"awesomeProject11/proxy"
	"awesomeProject11/state"
	"log"
	"net/http"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	allowedUser := map[string]string{"user": "pass"}

	myGlobalState := &state.GlobalState{
		UserMap:          map[string]*state.UserState{},
		ValidCredentials: allowedUser,
	}

	server := &proxy.ProxyServer{
		GlobalState: myGlobalState,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(server.ProxyHandler)))
}
