package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaBootstrapServers = getEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
	kafkaTopicVideo       = "claw_video"
	kafkaTopicComment     = "claw_comment"
	kafkaTopicAccount     = "claw_account"

	recordDir    = "sent_records"
	progressFile = "video_comment_progress.json"

	progressMu   sync.Mutex
	producerMu   sync.Mutex
	producer     *kafka.Writer
	producerOnce sync.Once
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetProducer returns the singleton Kafka producer
func GetProducer() *kafka.Writer {
	producerOnce.Do(func() {
		producer = &kafka.Writer{
			Addr:     kafka.TCP(kafkaBootstrapServers),
			Balancer: &kafka.LeastBytes{},
		}
	})
	return producer
}

// CloseProducer closes the Kafka producer
func CloseProducer() error {
	producerMu.Lock()
	defer producerMu.Unlock()
	if producer != nil {
		err := producer.Close()
		producer = nil
		return err
	}
	return nil
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0755)
}

// recordSentID appends an ID to a record file
func recordSentID(recordFile, idValue string) error {
	if err := EnsureDir(recordDir); err != nil {
		return err
	}
	filepath := filepath.Join(recordDir, recordFile)
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(idValue + "\n")
	return err
}

// loadSentIDs loads all IDs from a record file
func loadSentIDs(recordFile string) (map[string]struct{}, error) {
	filepath := filepath.Join(recordDir, recordFile)
	ids := make(map[string]struct{})

	f, err := os.Open(filepath)
	if os.IsNotExist(err) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			ids[line] = struct{}{}
		}
	}

	return ids, scanner.Err()
}

// SaveVideo saves a video to Kafka and records its BVID
func SaveVideo(video map[string]interface{}) error {
	bvid, ok := video["bvid"].(string)
	if !ok || bvid == "" {
		return fmt.Errorf("video has no bvid")
	}

	data, err := json.Marshal(video)
	if err != nil {
		return err
	}

	producer := GetProducer()
	err = producer.WriteMessages(context.Background(), kafka.Message{
		Topic: kafkaTopicVideo,
		Key:   []byte(bvid),
		Value: data,
	})
	if err != nil {
		return err
	}

	return recordSentID("sent_videos.txt", bvid)
}

// SaveComment saves a comment to Kafka and records its RPID
func SaveComment(comment map[string]interface{}) error {
	rpid := comment["rpid"]
	if rpid == nil {
		return fmt.Errorf("comment has no rpid")
	}

	rpidStr := fmt.Sprintf("%v", rpid)

	data, err := json.Marshal(comment)
	if err != nil {
		return err
	}

	producer := GetProducer()
	err = producer.WriteMessages(context.Background(), kafka.Message{
		Topic: kafkaTopicComment,
		Key:   []byte(rpidStr),
		Value: data,
	})
	if err != nil {
		return err
	}

	return recordSentID("sent_comments.txt", rpidStr)
}

// SaveAccount saves an account to Kafka and records its MID
func SaveAccount(account map[string]interface{}) error {
	card, ok := account["card"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("account has no card")
	}

	mid := card["mid"]
	if mid == nil {
		return fmt.Errorf("account has no mid")
	}

	midStr := fmt.Sprintf("%v", mid)

	data, err := json.Marshal(account)
	if err != nil {
		return err
	}

	producer := GetProducer()
	err = producer.WriteMessages(context.Background(), kafka.Message{
		Topic: kafkaTopicAccount,
		Key:   []byte(midStr),
		Value: data,
	})
	if err != nil {
		return err
	}

	return recordSentID("sent_accounts.txt", midStr)
}

// GetSavedVideoBvids returns all saved video BVIDs
func GetSavedVideoBvids() (map[string]struct{}, error) {
	return loadSentIDs("sent_videos.txt")
}

// GetSavedCommentRpids returns all saved comment RPIDs
func GetSavedCommentRpids() (map[string]struct{}, error) {
	return loadSentIDs("sent_comments.txt")
}

// GetSavedAccountMids returns all saved account MIDs
func GetSavedAccountMids() (map[string]struct{}, error) {
	return loadSentIDs("sent_accounts.txt")
}

// SavePendingMid saves a pending MID
func SavePendingMid(mid string) error {
	return recordSentID("pending_mids.txt", mid)
}

// GetPendingMids returns all pending MIDs
func GetPendingMids() (map[string]struct{}, error) {
	return loadSentIDs("pending_mids.txt")
}

// UpdatePendingMids updates the pending MIDs file with the remaining MIDs
func UpdatePendingMids(remainingMids map[string]struct{}) error {
	filepath := filepath.Join(recordDir, "pending_mids.txt")

	if len(remainingMids) == 0 {
		if _, err := os.Stat(filepath); err == nil {
			return os.Remove(filepath)
		}
		return nil
	}

	if err := EnsureDir(recordDir); err != nil {
		return err
	}

	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	for mid := range remainingMids {
		if _, err := f.WriteString(mid + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// VideoProgress represents the progress of comment crawling for a video
type VideoProgress struct {
	Done   bool   `json:"done"`
	Cursor string `json:"cursor"`
	Aid    int64  `json:"aid,omitempty"`
}

func getProgressFilepath() string {
	EnsureDir(recordDir)
	return filepath.Join(recordDir, progressFile)
}

func loadProgressData() (map[string]*VideoProgress, error) {
	filepath := getProgressFilepath()
	data := make(map[string]*VideoProgress)

	content, err := os.ReadFile(filepath)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, &data); err != nil {
		return make(map[string]*VideoProgress), nil
	}

	return data, nil
}

func saveProgressData(data map[string]*VideoProgress) error {
	filepath := getProgressFilepath()
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, content, 0644)
}

// SaveVideoCommentProgress saves the progress of comment crawling for a video
func SaveVideoCommentProgress(bvid, cursor string, aid int64) error {
	progressMu.Lock()
	defer progressMu.Unlock()

	data, err := loadProgressData()
	if err != nil {
		return err
	}

	if data[bvid] == nil {
		data[bvid] = &VideoProgress{Done: false, Cursor: ""}
	}
	data[bvid].Cursor = cursor
	if aid != 0 {
		data[bvid].Aid = aid
	}

	return saveProgressData(data)
}

// MarkVideoCommentsDone marks a video's comments as fully crawled
func MarkVideoCommentsDone(bvid string) error {
	progressMu.Lock()
	defer progressMu.Unlock()

	data, err := loadProgressData()
	if err != nil {
		return err
	}

	if data[bvid] == nil {
		data[bvid] = &VideoProgress{}
	}
	data[bvid].Done = true
	data[bvid].Cursor = ""

	return saveProgressData(data)
}

// GetVideoCommentProgress returns the progress of comment crawling for a video
func GetVideoCommentProgress(bvid string) (*VideoProgress, error) {
	progressMu.Lock()
	defer progressMu.Unlock()

	data, err := loadProgressData()
	if err != nil {
		return &VideoProgress{Done: false, Cursor: "", Aid: 0}, err
	}

	if progress, ok := data[bvid]; ok {
		return progress, nil
	}

	return &VideoProgress{Done: false, Cursor: "", Aid: 0}, nil
}

// LoadAllVideoProgress returns all video progress data
func LoadAllVideoProgress() (map[string]*VideoProgress, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	return loadProgressData()
}

// SetRecordDir sets the record directory (for testing)
func SetRecordDir(dir string) {
	recordDir = dir
}
