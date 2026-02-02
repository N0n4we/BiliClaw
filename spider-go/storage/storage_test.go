package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	SetRecordDir(tmpDir)
	return tmpDir
}

func TestRecordSentID(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Record some IDs
	if err := recordSentID("test.txt", "id1"); err != nil {
		t.Fatalf("Failed to record ID: %v", err)
	}
	if err := recordSentID("test.txt", "id2"); err != nil {
		t.Fatalf("Failed to record ID: %v", err)
	}

	// Verify file contents
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "id1\nid2\n"
	if string(content) != expected {
		t.Errorf("File content = %q, expected %q", string(content), expected)
	}
}

func TestLoadSentIDs(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "id1\nid2\nid3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load IDs
	ids, err := loadSentIDs("test.txt")
	if err != nil {
		t.Fatalf("Failed to load IDs: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("Expected 3 IDs, got %d", len(ids))
	}

	for _, id := range []string{"id1", "id2", "id3"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("Expected ID %s to be present", id)
		}
	}
}

func TestLoadSentIDs_NonExistent(t *testing.T) {
	setupTestDir(t)

	// Load from non-existent file
	ids, err := loadSentIDs("nonexistent.txt")
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(ids))
	}
}

func TestLoadSentIDs_EmptyLines(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a test file with empty lines
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "id1\n\nid2\n\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load IDs
	ids, err := loadSentIDs("test.txt")
	if err != nil {
		t.Fatalf("Failed to load IDs: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("Expected 2 IDs (empty lines ignored), got %d", len(ids))
	}
}

func TestVideoProgress(t *testing.T) {
	setupTestDir(t)

	// Save progress
	if err := SaveVideoCommentProgress("BV123", "cursor123", 12345); err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	// Get progress
	progress, err := GetVideoCommentProgress("BV123")
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}

	if progress.Cursor != "cursor123" {
		t.Errorf("Cursor = %s, expected cursor123", progress.Cursor)
	}
	if progress.Aid != 12345 {
		t.Errorf("Aid = %d, expected 12345", progress.Aid)
	}
	if progress.Done {
		t.Error("Expected Done to be false")
	}
}

func TestVideoProgress_MarkDone(t *testing.T) {
	setupTestDir(t)

	// Save initial progress
	if err := SaveVideoCommentProgress("BV123", "cursor123", 12345); err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	// Mark as done
	if err := MarkVideoCommentsDone("BV123"); err != nil {
		t.Fatalf("Failed to mark done: %v", err)
	}

	// Get progress
	progress, err := GetVideoCommentProgress("BV123")
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}

	if !progress.Done {
		t.Error("Expected Done to be true")
	}
	if progress.Cursor != "" {
		t.Errorf("Cursor should be empty after marking done, got %s", progress.Cursor)
	}
}

func TestVideoProgress_NonExistent(t *testing.T) {
	setupTestDir(t)

	// Get progress for non-existent video
	progress, err := GetVideoCommentProgress("BV_NONEXISTENT")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if progress.Done {
		t.Error("Expected Done to be false for non-existent video")
	}
	if progress.Cursor != "" {
		t.Error("Expected empty cursor for non-existent video")
	}
}

func TestLoadAllVideoProgress(t *testing.T) {
	setupTestDir(t)

	// Save multiple progress entries
	SaveVideoCommentProgress("BV1", "cursor1", 1)
	SaveVideoCommentProgress("BV2", "cursor2", 2)
	MarkVideoCommentsDone("BV3")

	// Load all
	all, err := LoadAllVideoProgress()
	if err != nil {
		t.Fatalf("Failed to load all progress: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(all))
	}

	if all["BV1"].Cursor != "cursor1" {
		t.Error("BV1 cursor mismatch")
	}
	if all["BV2"].Cursor != "cursor2" {
		t.Error("BV2 cursor mismatch")
	}
	if !all["BV3"].Done {
		t.Error("BV3 should be done")
	}
}

func TestPendingMids(t *testing.T) {
	setupTestDir(t)

	// Save pending MIDs
	SavePendingMid("123")
	SavePendingMid("456")
	SavePendingMid("789")

	// Get pending MIDs
	mids, err := GetPendingMids()
	if err != nil {
		t.Fatalf("Failed to get pending MIDs: %v", err)
	}

	if len(mids) != 3 {
		t.Errorf("Expected 3 MIDs, got %d", len(mids))
	}
}

func TestUpdatePendingMids(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Save initial MIDs
	SavePendingMid("123")
	SavePendingMid("456")

	// Update with remaining MIDs
	remaining := map[string]struct{}{
		"456": {},
		"789": {},
	}
	if err := UpdatePendingMids(remaining); err != nil {
		t.Fatalf("Failed to update pending MIDs: %v", err)
	}

	// Verify
	mids, err := GetPendingMids()
	if err != nil {
		t.Fatalf("Failed to get pending MIDs: %v", err)
	}

	if len(mids) != 2 {
		t.Errorf("Expected 2 MIDs, got %d", len(mids))
	}
	if _, ok := mids["456"]; !ok {
		t.Error("Expected 456 to be present")
	}
	if _, ok := mids["789"]; !ok {
		t.Error("Expected 789 to be present")
	}

	// Update with empty set (should remove file)
	if err := UpdatePendingMids(map[string]struct{}{}); err != nil {
		t.Fatalf("Failed to update with empty set: %v", err)
	}

	// File should be removed
	if _, err := os.Stat(filepath.Join(tmpDir, "pending_mids.txt")); !os.IsNotExist(err) {
		t.Error("Expected file to be removed")
	}
}

func TestGetSavedFunctions(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "sent_videos.txt"), []byte("BV1\nBV2\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sent_comments.txt"), []byte("123\n456\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sent_accounts.txt"), []byte("mid1\nmid2\n"), 0644)

	// Test GetSavedVideoBvids
	bvids, err := GetSavedVideoBvids()
	if err != nil {
		t.Fatalf("Failed to get saved BVIDs: %v", err)
	}
	if len(bvids) != 2 {
		t.Errorf("Expected 2 BVIDs, got %d", len(bvids))
	}

	// Test GetSavedCommentRpids
	rpids, err := GetSavedCommentRpids()
	if err != nil {
		t.Fatalf("Failed to get saved RPIDs: %v", err)
	}
	if len(rpids) != 2 {
		t.Errorf("Expected 2 RPIDs, got %d", len(rpids))
	}

	// Test GetSavedAccountMids
	mids, err := GetSavedAccountMids()
	if err != nil {
		t.Fatalf("Failed to get saved MIDs: %v", err)
	}
	if len(mids) != 2 {
		t.Errorf("Expected 2 MIDs, got %d", len(mids))
	}
}
