package utils

import (
	"encoding/base64"
	"testing"
)

func TestGenerateSecureString(t *testing.T) {
	// Test that it generates the correct length
	length := 32
	str, err := GenerateSecureString(length)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	decoded, err := base64.URLEncoding.DecodeString(str)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(decoded) != length {
		t.Errorf("Expected length %d, got %d", length, len(str))
	}

	// Test for uniqueness (call it twice, they shouldn't match)
	str2, _ := GenerateSecureString(length)
	if str == str2 {
		t.Error("Generated strings are not unique; security risk identified")
	}
}
