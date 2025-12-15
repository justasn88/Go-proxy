package auth

import (
	"net/http"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	creds := map[string]string{
		"admin": "secret",
	}

	tests := []struct {
		name           string
		headerValue    string
		wantUser       string
		wantAuthorized bool
	}{
		{
			name:           "Teisingi duomenys",
			headerValue:    "Basic YWRtaW46c2VjcmV0",
			wantUser:       "admin",
			wantAuthorized: true,
		},
		{
			name:           "Blogas slaptažodis",
			headerValue:    "Basic YWRtaW46YmxvZ2Fz",
			wantUser:       "",
			wantAuthorized: false,
		},
		{
			name:           "Nėra headerio",
			headerValue:    "",
			wantUser:       "",
			wantAuthorized: false,
		},
		{
			name:           "Blogas formatas",
			headerValue:    "Bearer token123",
			wantUser:       "",
			wantAuthorized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			if tt.headerValue != "" {
				req.Header.Set("Proxy-Authorization", tt.headerValue)
			}

			user, auth := Authenticate(req, creds)

			if auth != tt.wantAuthorized {
				t.Errorf("Authenticate() authorized = %v, norėjome %v", auth, tt.wantAuthorized)
			}
			if user != tt.wantUser {
				t.Errorf("Authenticate() user = %v, norėjome %v", user, tt.wantUser)
			}
		})
	}
}
