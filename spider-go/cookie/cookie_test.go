package cookie

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cookies.json")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	return configPath
}

func TestCookieItem_MarkFailed(t *testing.T) {
	cookie := &CookieItem{
		Value:    "test_cookie",
		Name:     "test",
		Enabled:  true,
		IsValid:  true,
		MaxFails: 3,
	}

	// First two failures should not disable
	if cookie.MarkFailed() {
		t.Error("Cookie should not be disabled after 1 failure")
	}
	if cookie.MarkFailed() {
		t.Error("Cookie should not be disabled after 2 failures")
	}

	// Third failure should disable
	if !cookie.MarkFailed() {
		t.Error("Cookie should be disabled after 3 failures")
	}
	if cookie.IsValid {
		t.Error("Cookie should be marked as invalid")
	}
}

func TestCookieItem_Reset(t *testing.T) {
	cookie := &CookieItem{
		Value:     "test_cookie",
		IsValid:   false,
		FailCount: 5,
	}

	cookie.Reset()

	if cookie.FailCount != 0 {
		t.Errorf("Expected FailCount to be 0, got %d", cookie.FailCount)
	}
	if !cookie.IsValid {
		t.Error("Expected IsValid to be true")
	}
}

func TestCookiePool_LoadCookies(t *testing.T) {
	config := `{
		"cookies": [
			{"value": "cookie1", "name": "账号1", "enabled": true},
			{"value": "cookie2", "name": "账号2", "enabled": true},
			{"value": "cookie3", "name": "账号3", "enabled": false}
		],
		"settings": {
			"strategy": "round_robin",
			"validate_on_load": false
		}
	}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	status := pool.GetStatus()
	if status["total"].(int) != 2 {
		t.Errorf("Expected 2 cookies loaded, got %d", status["total"].(int))
	}
	if status["strategy"].(string) != "round_robin" {
		t.Errorf("Expected strategy 'round_robin', got %s", status["strategy"].(string))
	}
}

func TestCookiePool_RoundRobin(t *testing.T) {
	config := `{
		"cookies": [
			{"value": "cookie1", "name": "账号1", "enabled": true},
			{"value": "cookie2", "name": "账号2", "enabled": true},
			{"value": "cookie3", "name": "账号3", "enabled": true}
		],
		"settings": {"strategy": "round_robin"}
	}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	// Round robin should cycle through cookies
	c1 := pool.GetCookie()
	c2 := pool.GetCookie()
	c3 := pool.GetCookie()
	c4 := pool.GetCookie()

	if c1 != "cookie1" {
		t.Errorf("Expected first cookie to be 'cookie1', got %s", c1)
	}
	if c2 != "cookie2" {
		t.Errorf("Expected second cookie to be 'cookie2', got %s", c2)
	}
	if c3 != "cookie3" {
		t.Errorf("Expected third cookie to be 'cookie3', got %s", c3)
	}
	if c4 != "cookie1" {
		t.Errorf("Expected fourth cookie to be 'cookie1' (wrap around), got %s", c4)
	}
}

func TestCookiePool_Random(t *testing.T) {
	config := `{
		"cookies": [
			{"value": "cookie1", "name": "账号1", "enabled": true},
			{"value": "cookie2", "name": "账号2", "enabled": true}
		],
		"settings": {"strategy": "random"}
	}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	// Just verify we get valid cookies
	for i := 0; i < 10; i++ {
		c := pool.GetCookie()
		if c != "cookie1" && c != "cookie2" {
			t.Errorf("Got unexpected cookie: %s", c)
		}
	}
}

func TestCookiePool_MarkInvalid(t *testing.T) {
	config := `{
		"cookies": [
			{"value": "cookie1", "name": "账号1", "enabled": true},
			{"value": "cookie2", "name": "账号2", "enabled": true}
		],
		"settings": {"strategy": "round_robin"}
	}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	// Mark cookie1 as invalid permanently
	pool.MarkInvalid("cookie1", true)

	// Should only get cookie2 now
	for i := 0; i < 5; i++ {
		c := pool.GetCookie()
		if c != "cookie2" {
			t.Errorf("Expected only 'cookie2', got %s", c)
		}
	}
}

func TestCookiePool_MarkInvalid_Temporary(t *testing.T) {
	config := `{
		"cookies": [
			{"value": "cookie1", "name": "账号1", "enabled": true}
		],
		"settings": {"strategy": "round_robin"}
	}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	// Mark as failed twice (should still be valid)
	pool.MarkInvalid("cookie1", false)
	pool.MarkInvalid("cookie1", false)

	if pool.Len() != 1 {
		t.Error("Cookie should still be available after 2 failures")
	}

	// Third failure should disable
	pool.MarkInvalid("cookie1", false)

	if pool.Len() != 0 {
		t.Error("Cookie should be disabled after 3 failures")
	}
}

func TestCookiePool_EmptyPool(t *testing.T) {
	config := `{"cookies": [], "settings": {}}`

	configPath := createTempConfig(t, config)
	pool := NewCookiePool(configPath)

	c := pool.GetCookie()
	if c != "" {
		t.Errorf("Expected empty string from empty pool, got %s", c)
	}

	item := pool.GetCookieItem()
	if item != nil {
		t.Error("Expected nil from empty pool")
	}
}

func TestCookiePool_NonExistentConfig(t *testing.T) {
	pool := NewCookiePool("/nonexistent/path/cookies.json")

	if pool.Len() != 0 {
		t.Error("Expected empty pool for non-existent config")
	}
}

func TestIsCookieError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{-101, true},  // Not logged in
		{-352, true},  // Risk control
		{-412, true},  // Request blocked
		{0, false},    // Success
		{-400, false}, // Other error
	}

	for _, tt := range tests {
		result := IsCookieError(tt.code)
		if result != tt.expected {
			t.Errorf("IsCookieError(%d) = %v, expected %v", tt.code, result, tt.expected)
		}
	}
}
