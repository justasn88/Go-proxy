package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func Authenticate(r *http.Request, validCredentials map[string]string) (username string, authorized bool) {
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

	if allowedPass, ok := validCredentials[user]; ok && allowedPass == pass {
		return user, true
	}
	return "", false
}
