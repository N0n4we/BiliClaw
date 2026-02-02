package crawler

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStats_Concurrent(t *testing.T) {
	stats := &Stats{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats.incVideosSaved()
			stats.incCommentsSaved()
			stats.incRepliesSaved()
			stats.incAccountsSaved()
		}()
	}
	wg.Wait()

	if stats.VideosSaved != 100 {
		t.Errorf("VideosSaved = %d, expected 100", stats.VideosSaved)
	}
	if stats.CommentsSaved != 100 {
		t.Errorf("CommentsSaved = %d, expected 100", stats.CommentsSaved)
	}
	if stats.RepliesSaved != 100 {
		t.Errorf("RepliesSaved = %d, expected 100", stats.RepliesSaved)
	}
	if stats.AccountsSaved != 100 {
		t.Errorf("AccountsSaved = %d, expected 100", stats.AccountsSaved)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.NThreads != 3 {
		t.Errorf("NThreads = %d, expected 3", config.NThreads)
	}
	if config.PagesPerThread != 2 {
		t.Errorf("PagesPerThread = %d, expected 2", config.PagesPerThread)
	}
	if config.DelayMin != 2.0 {
		t.Errorf("DelayMin = %f, expected 2.0", config.DelayMin)
	}
	if config.DelayMax != 4.0 {
		t.Errorf("DelayMax = %f, expected 4.0", config.DelayMax)
	}
	if !config.Resume {
		t.Error("Resume should be true by default")
	}
	if !config.ResumePendingMids {
		t.Error("ResumePendingMids should be true by default")
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"keyword": "测试关键词",
		"n_threads": 5,
		"pages_per_thread": 3,
		"delay_min": 1.0,
		"delay_max": 2.0,
		"resume": false
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Keyword != "测试关键词" {
		t.Errorf("Keyword = %s, expected 测试关键词", config.Keyword)
	}
	if config.NThreads != 5 {
		t.Errorf("NThreads = %d, expected 5", config.NThreads)
	}
	if config.PagesPerThread != 3 {
		t.Errorf("PagesPerThread = %d, expected 3", config.PagesPerThread)
	}
	if config.DelayMin != 1.0 {
		t.Errorf("DelayMin = %f, expected 1.0", config.DelayMin)
	}
	if config.Resume {
		t.Error("Resume should be false")
	}
	// Default values should be preserved for unspecified fields
	if config.CookieConfigPath != "cookies.json" {
		t.Errorf("CookieConfigPath should use default, got %s", config.CookieConfigPath)
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestBiliCrawler_AddUserMid(t *testing.T) {
	config := DefaultConfig()
	config.Resume = false

	crawler := &BiliCrawler{
		config:       config,
		userMidQueue: make(chan string, 10),
		userMids:     make(map[string]struct{}),
		savedMids:    make(map[string]struct{}),
	}

	// Add first MID
	crawler.addUserMid("123")
	if len(crawler.userMids) != 1 {
		t.Errorf("Expected 1 MID, got %d", len(crawler.userMids))
	}

	// Add same MID again (should be deduplicated)
	crawler.addUserMid("123")
	if len(crawler.userMids) != 1 {
		t.Errorf("Expected 1 MID after duplicate, got %d", len(crawler.userMids))
	}

	// Add different MID
	crawler.addUserMid("456")
	if len(crawler.userMids) != 2 {
		t.Errorf("Expected 2 MIDs, got %d", len(crawler.userMids))
	}
}

func TestBiliCrawler_BvidTracking(t *testing.T) {
	crawler := &BiliCrawler{
		savedBvids: make(map[string]struct{}),
		mu:         sync.Mutex{},
	}

	// Initially not saved
	if crawler.isBvidSaved("BV123") {
		t.Error("BV123 should not be saved initially")
	}

	// Mark as saved
	crawler.markBvidSaved("BV123")

	// Now should be saved
	if !crawler.isBvidSaved("BV123") {
		t.Error("BV123 should be saved after marking")
	}
}

func TestBiliCrawler_RpidTracking(t *testing.T) {
	crawler := &BiliCrawler{
		savedRpids: make(map[string]struct{}),
		mu:         sync.Mutex{},
	}

	// Initially not saved
	if crawler.isRpidSaved("12345") {
		t.Error("RPID should not be saved initially")
	}

	// Mark as saved
	crawler.markRpidSaved("12345")

	// Now should be saved
	if !crawler.isRpidSaved("12345") {
		t.Error("RPID should be saved after marking")
	}
}

func TestBiliCrawler_MidTracking(t *testing.T) {
	crawler := &BiliCrawler{
		savedMids: make(map[string]struct{}),
		mu:        sync.Mutex{},
	}

	// Initially not saved
	if crawler.isMidSaved("123") {
		t.Error("MID should not be saved initially")
	}

	// Mark as saved
	crawler.markMidSaved("123")

	// Now should be saved
	if !crawler.isMidSaved("123") {
		t.Error("MID should be saved after marking")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestBoolToStr(t *testing.T) {
	if boolToStr(true, "yes", "no") != "yes" {
		t.Error("boolToStr(true) should return trueStr")
	}
	if boolToStr(false, "yes", "no") != "no" {
		t.Error("boolToStr(false) should return falseStr")
	}
}

func TestVideoTask(t *testing.T) {
	task := &VideoTask{
		Detail: map[string]interface{}{
			"bvid":  "BV123",
			"title": "Test Video",
		},
	}

	if task.Detail["bvid"] != "BV123" {
		t.Error("VideoTask should store detail correctly")
	}
}

func TestCommentTask(t *testing.T) {
	task := &CommentTask{
		Aid: 12345,
		Comment: map[string]interface{}{
			"rpid":    float64(67890),
			"content": "Test comment",
		},
	}

	if task.Aid != 12345 {
		t.Error("CommentTask should store Aid correctly")
	}
	if task.Comment["rpid"] != float64(67890) {
		t.Error("CommentTask should store Comment correctly")
	}
}

func TestChannelCommunication(t *testing.T) {
	videoQueue := make(chan *VideoTask, 10)
	commentQueue := make(chan *CommentTask, 10)

	// Test video queue
	videoQueue <- &VideoTask{Detail: map[string]interface{}{"bvid": "BV1"}}
	videoQueue <- &VideoTask{Detail: map[string]interface{}{"bvid": "BV2"}}

	task1 := <-videoQueue
	task2 := <-videoQueue

	if task1.Detail["bvid"] != "BV1" || task2.Detail["bvid"] != "BV2" {
		t.Error("Video queue should maintain order")
	}

	// Test comment queue
	commentQueue <- &CommentTask{Aid: 1, Comment: map[string]interface{}{"rpid": float64(1)}}
	commentQueue <- &CommentTask{Aid: 2, Comment: map[string]interface{}{"rpid": float64(2)}}

	ct1 := <-commentQueue
	ct2 := <-commentQueue

	if ct1.Aid != 1 || ct2.Aid != 2 {
		t.Error("Comment queue should maintain order")
	}
}
