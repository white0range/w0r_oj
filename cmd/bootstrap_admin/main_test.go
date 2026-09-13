package main

import "testing"

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{name: "valid", username: "admin", password: "a-strong-password", wantErr: false},
		{name: "short username", username: "ad", password: "a-strong-password", wantErr: true},
		{name: "short password", username: "admin", password: "short", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredentials(tt.username, tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCredentials() error = %v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
