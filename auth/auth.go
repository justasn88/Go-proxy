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
	user, pass, found := strings.Cut(credentials, ":")

	if !found {
		return "", false
	}

	if allowedPass, ok := validCredentials[user]; ok && allowedPass == pass {
		return user, true
	}
	return "", false
}
