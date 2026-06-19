package iam

import "testing"

func TestValidatePassword(t *testing.T) {
	defaultPolicy := passwordPolicyState{MinLength: 12}

	tests := []struct {
		password string
		wantOK   bool
	}{
		{"Password1234", true},
		{"Abcd12345678", true},
		{"p1!xxxxxxxxx", true},
		{"Ab3$%^&*xxxx", true},
		{"", false},               // empty
		{"Password1", false},      // too short (< 12)
		{"abcdefghijkl", false},   // no digit
		{"123456789012", false},   // no letter
		{"!!@@##$$!!!!", false},   // no letter, no digit
		{"pass word123", false},   // space is invalid
		{"パスワード12345abcd", false}, // non-ASCII letter
	}
	for _, tt := range tests {
		msg := validatePassword(tt.password, defaultPolicy)
		gotOK := msg == ""
		if gotOK != tt.wantOK {
			t.Errorf("validatePassword(%q) = %q, wantOK=%v", tt.password, msg, tt.wantOK)
		}
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	t.Run("require_uppercase", func(t *testing.T) {
		policy := passwordPolicyState{MinLength: 12, RequireUppercase: true}
		if msg := validatePassword("abcdefghijk1", policy); msg == "" {
			t.Error("expected error for missing uppercase")
		}
		if msg := validatePassword("Abcdefghijk1", policy); msg != "" {
			t.Errorf("unexpected error: %s", msg)
		}
	})

	t.Run("require_lowercase", func(t *testing.T) {
		policy := passwordPolicyState{MinLength: 12, RequireLowercase: true}
		if msg := validatePassword("ABCDEFGHIJK1", policy); msg == "" {
			t.Error("expected error for missing lowercase")
		}
		if msg := validatePassword("ABCDEFGHIJk1", policy); msg != "" {
			t.Errorf("unexpected error: %s", msg)
		}
	})

	t.Run("require_symbols", func(t *testing.T) {
		policy := passwordPolicyState{MinLength: 12, RequireSymbols: true}
		if msg := validatePassword("Abcdefghijk1", policy); msg == "" {
			t.Error("expected error for missing symbol")
		}
		if msg := validatePassword("Abcdefghij1!", policy); msg != "" {
			t.Errorf("unexpected error: %s", msg)
		}
	})

	t.Run("custom_min_length", func(t *testing.T) {
		policy := passwordPolicyState{MinLength: 16}
		if msg := validatePassword("Abcdefghijk12", policy); msg == "" {
			t.Error("expected error for password shorter than 16")
		}
		if msg := validatePassword("Abcdefghijklmn1!", policy); msg != "" {
			t.Errorf("unexpected error: %s", msg)
		}
	})
}
