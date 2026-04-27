package api

import "testing"

func TestValidAPIKey(t *testing.T) {
	t.Parallel()

	if !validAPIKey("secret-key", "secret-key") {
		t.Fatal("expected matching keys to validate")
	}
	if validAPIKey("wrong-key", "secret-key") {
		t.Fatal("expected mismatched keys to fail")
	}
	if validAPIKey("", "secret-key") {
		t.Fatal("expected empty key to fail")
	}
	if !validAPIKey("", "") {
		t.Fatal("expected empty expected key to disable auth")
	}
}
