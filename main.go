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
	state.GlobalStateInstance = state.GlobalState{
		UserMap:          map[string]*state.UserState{},
		ValidCredentials: allowedUser,
	}

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(proxy.ProxyHandler)))
}
