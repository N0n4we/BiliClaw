package api

import (
	"testing"
)

func TestMd5Hash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"world", "7d793037a0760186574b0282f2f435e7"},
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
	}

	for _, tt := range tests {
		result := md5Hash(tt.input)
		if result != tt.expected {
			t.Errorf("md5Hash(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetMixinKey(t *testing.T) {
	// Test with a known input
	// The encoding table rearranges characters from the original string
	orig := "7cd084941338484aae1ad9425b84077c4932caff0ff746eab6f01bf08b70ac45"

	result := getMixinKey(orig)

	// Result should be 32 characters
	if len(result) != 32 {
		t.Errorf("getMixinKey result length = %d, expected 32", len(result))
	}

	// Verify the encoding is deterministic
	result2 := getMixinKey(orig)
	if result != result2 {
		t.Error("getMixinKey should be deterministic")
	}
}

func TestGetMixinKey_ShortInput(t *testing.T) {
	// Test with input shorter than expected
	orig := "short"

	result := getMixinKey(orig)

	// Should not panic and return what it can
	if result == "" {
		t.Error("getMixinKey should handle short input")
	}
}

func TestWbiMixinKeyEncTab(t *testing.T) {
	// Verify the encoding table has 64 entries
	if len(wbiMixinKeyEncTab) != 64 {
		t.Errorf("wbiMixinKeyEncTab length = %d, expected 64", len(wbiMixinKeyEncTab))
	}

	// Verify all values are in range [0, 63]
	for i, v := range wbiMixinKeyEncTab {
		if v < 0 || v > 63 {
			t.Errorf("wbiMixinKeyEncTab[%d] = %d, expected value in [0, 63]", i, v)
		}
	}

	// Verify all values are unique (it's a permutation)
	seen := make(map[int]bool)
	for _, v := range wbiMixinKeyEncTab {
		if seen[v] {
			t.Errorf("wbiMixinKeyEncTab contains duplicate value: %d", v)
		}
		seen[v] = true
	}
}

func TestGenerateWbiSign(t *testing.T) {
	// Test that GenerateWbiSign produces consistent results
	params := map[string]string{
		"oid":  "12345",
		"type": "1",
		"mode": "2",
	}

	wRid1, wts1 := GenerateWbiSign(params, nil)
	wRid2, wts2 := GenerateWbiSign(params, nil)

	// wts should be close (within 1 second)
	if wts2-wts1 > 1 {
		t.Error("wts values should be close in time")
	}

	// wRid should be 32 character hex string
	if len(wRid1) != 32 {
		t.Errorf("wRid length = %d, expected 32", len(wRid1))
	}

	// Verify it's a valid hex string
	for _, c := range wRid1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("wRid contains invalid character: %c", c)
		}
	}

	// If called at the same second with same params, should produce same result
	// (since wts is part of the signature)
	if wts1 == wts2 && wRid1 != wRid2 {
		t.Error("Same params and wts should produce same wRid")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, expected 3", config.MaxRetries)
	}
	if config.BaseDelay != 1.0 {
		t.Errorf("BaseDelay = %f, expected 1.0", config.BaseDelay)
	}
	if config.MaxDelay != 30.0 {
		t.Errorf("MaxDelay = %f, expected 30.0", config.MaxDelay)
	}
}

func TestDefaultHeaders(t *testing.T) {
	headers := getDefaultHeaders()
	if headers["User-Agent"] == "" {
		t.Error("User-Agent header should not be empty")
	}
	if headers["Referer"] != "https://www.bilibili.com" {
		t.Error("Referer should be bilibili.com")
	}
	if headers["Accept"] == "" {
		t.Error("Accept header should not be empty")
	}
}

func TestSession_Headers(t *testing.T) {
	// Test that session properly copies headers
	session := &Session{
		headers: make(map[string]string),
	}

	for k, v := range getDefaultHeaders() {
		session.headers[k] = v
	}
	session.headers["Cookie"] = "test_cookie"

	if session.headers["Cookie"] != "test_cookie" {
		t.Error("Session should have cookie set")
	}
	if session.headers["User-Agent"] == "" {
		t.Error("Session should have User-Agent set")
	}
}

func TestSetUserAgent(t *testing.T) {
	originalUA := GetUserAgent()

	// Set custom UA
	SetUserAgent("CustomAgent/1.0")
	if GetUserAgent() != "CustomAgent/1.0" {
		t.Error("SetUserAgent should update the User-Agent")
	}

	// Verify headers use new UA
	headers := getDefaultHeaders()
	if headers["User-Agent"] != "CustomAgent/1.0" {
		t.Error("getDefaultHeaders should use the new User-Agent")
	}

	// Restore original
	SetUserAgent(originalUA)
}
