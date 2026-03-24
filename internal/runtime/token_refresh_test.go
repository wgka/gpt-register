package runtime

import (
	"net/http"
	"testing"
)

func TestShouldDeleteAccountOnValidationStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "ok", status: http.StatusOK, want: false},
		{name: "unauthorized", status: http.StatusUnauthorized, want: true},
		{name: "forbidden", status: http.StatusForbidden, want: true},
		{name: "server error", status: http.StatusInternalServerError, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDeleteAccountOnValidationStatus(tt.status); got != tt.want {
				t.Fatalf("shouldDeleteAccountOnValidationStatus(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
