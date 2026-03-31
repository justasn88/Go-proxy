package auth

import (
	"net/http"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name         string
		headerValue  string
		wantUser     string
		wantPassword string
		wantFound    bool
	}{
		{
			name:         "Teisingi duomenys",
			headerValue:  "Basic YWRtaW46c2VjcmV0", // admin:secret
			wantUser:     "admin",
			wantPassword: "secret",
			wantFound:    true,
		},
		{
			name:         "Nėra headerio",
			headerValue:  "",
			wantUser:     "",
			wantPassword: "",
			wantFound:    false,
		},
		{
			name:         "Blogas formatas",
			headerValue:  "Bearer token123",
			wantUser:     "",
			wantPassword: "",
			wantFound:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			if tt.headerValue != "" {
				req.Header.Set("Proxy-Authorization", tt.headerValue)
			}

			user, pass, found := ExtractCredentials(req)

			if found != tt.wantFound {
				t.Errorf("ExtractCredentials() found = %v, norėjome %v", found, tt.wantFound)
			}
			if user != tt.wantUser {
				t.Errorf("ExtractCredentials() user = %v, norėjome %v", user, tt.wantUser)
			}
			if pass != tt.wantPassword {
				t.Errorf("ExtractCredentials() pass = %v, norėjome %v", pass, tt.wantPassword)
			}
		})
	}
}
