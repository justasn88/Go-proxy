package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func ExtractCredentials(r *http.Request) (username string, password string, ok bool) {
	authHeader := r.Header.Get("Proxy-Authorization")
	if authHeader == "" {
		return "", "", false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Basic" {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}

	credentials := string(decoded)
	user, pass, found := strings.Cut(credentials, ":")

	return user, pass, found
}
