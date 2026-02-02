package crawler

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"spider-go/api"
	"spider-go/ratelimit"
	"spider-go/storage"
)

// Config holds the crawler configuration
type Config struct {
	Keyword           string  `json:"keyword"`
	NThreads          int     `json:"n_threads"`
	PagesPerThread    int     `json:"pages_per_thread"`
	VideoDir          string  `json:"video_dir"`
	CommentDir        string  `json:"comment_dir"`
	AccountDir        string  `json:"account_dir"`
	DelayMin          float64 `json:"delay_min"`
	DelayMax          float64 `json:"delay_max"`
	Resume            bool    `json:"resume"`
	ResumePendingMids bool    `json:"resume_pending_mids"`
	CookieConfigPath  string  `json:"cookie_config_path"`
	RateLimitRate     float64 `json:"rate_limit_rate"`
	RateLimitCapacity float64 `json:"rate_limit_capacity"`
	UserAgent         string  `json:"user_agent"`
}

// DefaultConfig returns the default crawler configuration
func DefaultConfig() Config {
	return Config{
		Keyword:           "",
		NThreads:          3,
		PagesPerThread:    2,
		VideoDir:          "videos",
		CommentDir:        "comments",
		AccountDir:        "accounts",
		DelayMin:          2.0,
		DelayMax:          4.0,
		Resume:            true,
		ResumePendingMids: true,
		CookieConfigPath:  "cookies.json",
		RateLimitRate:     2.0,
		RateLimitCapacity: 5.0,
		UserAgent:         "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0",
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// VideoTask represents a video to be processed
type VideoTask struct {
	Detail map[string]interface{}
}

// CommentTask represents a comment with replies to be processed
type CommentTask struct {
	Aid     int64
	Comment map[string]interface{}
}

// Stats holds crawler statistics
type Stats struct {
	VideosSaved     int
	CommentsSaved   int
	RepliesSaved    int
	AccountsSaved   int
	VideosSkipped   int
	CommentsSkipped int
	AccountsSkipped int
	mu              sync.Mutex
}

func (s *Stats) incVideosSaved() {
	s.mu.Lock()
	s.VideosSaved++
	s.mu.Unlock()
}

func (s *Stats) incCommentsSaved() {
	s.mu.Lock()
	s.CommentsSaved++
	s.mu.Unlock()
}

func (s *Stats) incRepliesSaved() {
	s.mu.Lock()
	s.RepliesSaved++
	s.mu.Unlock()
}

func (s *Stats) incAccountsSaved() {
	s.mu.Lock()
	s.AccountsSaved++
	s.mu.Unlock()
}

func (s *Stats) incVideosSkipped() {
	s.mu.Lock()
	s.VideosSkipped++
	s.mu.Unlock()
}

func (s *Stats) incCommentsSkipped() {
	s.mu.Lock()
	s.CommentsSkipped++
	s.mu.Unlock()
}

func (s *Stats) incAccountsSkipped() {
	s.mu.Lock()
	s.AccountsSkipped++
	s.mu.Unlock()
}

// BiliCrawler is the main crawler engine
type BiliCrawler struct {
	config Config
	stats  Stats

	videoQueue   chan *VideoTask
	commentQueue chan *CommentTask
	userMidQueue chan string

	userMids   map[string]struct{}
	savedBvids map[string]struct{}
	savedRpids map[string]struct{}
	savedMids  map[string]struct{}

	videoProgress map[string]*storage.VideoProgress

	mu sync.Mutex
}

// NewBiliCrawler creates a new crawler instance
func NewBiliCrawler(config Config) (*BiliCrawler, error) {
	// Initialize rate limiter with config values
	ratelimit.InitRateLimiter(config.RateLimitRate, config.RateLimitCapacity)

	// Set User-Agent
	if config.UserAgent != "" {
		api.SetUserAgent(config.UserAgent)
	}

	crawler := &BiliCrawler{
		config:       config,
		videoQueue:   make(chan *VideoTask, 100),
		commentQueue: make(chan *CommentTask, 500),
		userMidQueue: make(chan string, 1000),
		userMids:     make(map[string]struct{}),
		savedBvids:   make(map[string]struct{}),
		savedRpids:   make(map[string]struct{}),
		savedMids:    make(map[string]struct{}),
	}

	if config.Resume {
		var err error
		crawler.savedBvids, err = storage.GetSavedVideoBvids()
		if err != nil {
			return nil, fmt.Errorf("failed to load saved BVIDs: %w", err)
		}

		crawler.savedRpids, err = storage.GetSavedCommentRpids()
		if err != nil {
			return nil, fmt.Errorf("failed to load saved RPIDs: %w", err)
		}

		crawler.savedMids, err = storage.GetSavedAccountMids()
		if err != nil {
			return nil, fmt.Errorf("failed to load saved MIDs: %w", err)
		}

		crawler.videoProgress, err = storage.LoadAllVideoProgress()
		if err != nil {
			return nil, fmt.Errorf("failed to load video progress: %w", err)
		}
	} else {
		crawler.videoProgress = make(map[string]*storage.VideoProgress)
	}

	return crawler, nil
}

func (c *BiliCrawler) delay() {
	d := c.config.DelayMin + rand.Float64()*(c.config.DelayMax-c.config.DelayMin)
	time.Sleep(time.Duration(d * float64(time.Second)))
}

func (c *BiliCrawler) addUserMid(mid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.userMids[mid]; exists {
		return
	}

	c.userMids[mid] = struct{}{}

	if c.config.Resume {
		if _, saved := c.savedMids[mid]; saved {
			return
		}
	}

	storage.SavePendingMid(mid)
	select {
	case c.userMidQueue <- mid:
	default:
		// Queue full, skip
	}
}

func (c *BiliCrawler) isBvidSaved(bvid string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exists := c.savedBvids[bvid]
	return exists
}

func (c *BiliCrawler) markBvidSaved(bvid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.savedBvids[bvid] = struct{}{}
}

func (c *BiliCrawler) isRpidSaved(rpid string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exists := c.savedRpids[rpid]
	return exists
}

func (c *BiliCrawler) markRpidSaved(rpid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.savedRpids[rpid] = struct{}{}
}

func (c *BiliCrawler) isMidSaved(mid string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exists := c.savedMids[mid]
	return exists
}

func (c *BiliCrawler) markMidSaved(mid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.savedMids[mid] = struct{}{}
}

func (c *BiliCrawler) searchWorker(threadID int, pagesPerThread int, results chan<- map[string]interface{}, wg *sync.WaitGroup, session *api.Session) {
	defer wg.Done()

	for page := 1; page <= pagesPerThread; page++ {
		actualPage := threadID*pagesPerThread + page
		fmt.Printf("[搜索线程%d] 正在获取第 %d 页...\n", threadID, actualPage)

		result, err := api.SearchVideos(c.config.Keyword, actualPage, 50, session, c.config.CookieConfigPath)
		if err != nil {
			fmt.Printf("[搜索线程%d] 第 %d 页错误: %v\n", threadID, actualPage, err)
		} else {
			for _, video := range result.Videos {
				results <- video
			}
			fmt.Printf("[搜索线程%d] 第 %d 页获取 %d 条视频\n", threadID, actualPage, len(result.Videos))
		}
		c.delay()
	}
}

func (c *BiliCrawler) videoDetailWorker(threadID int, videos <-chan map[string]interface{}, wg *sync.WaitGroup, session *api.Session) {
	defer wg.Done()

	for video := range videos {
		bvid, ok := video["bvid"].(string)
		if !ok {
			continue
		}

		detail, err := api.GetVideoDetail(bvid, session, c.config.CookieConfigPath)
		if err != nil {
			fmt.Printf("[视频线程%d] %s 获取详情失败: %v\n", threadID, bvid, err)
		} else {
			detail["topic_keyword"] = c.config.Keyword

			if err := storage.SaveVideo(detail); err == nil {
				c.stats.incVideosSaved()
				c.markBvidSaved(bvid)

				if owner, ok := detail["owner"].(map[string]interface{}); ok {
					if mid, ok := owner["mid"]; ok {
						c.addUserMid(fmt.Sprintf("%v", mid))
					}
				}

				c.videoQueue <- &VideoTask{Detail: detail}
				fmt.Printf("[视频线程%d] %s 已保存并推送到评论队列\n", threadID, bvid)
			}
		}
		c.delay()
	}
}

func (c *BiliCrawler) commentWorker(threadID int, wg *sync.WaitGroup, done <-chan struct{}, session *api.Session) {
	defer wg.Done()

	for {
		select {
		case <-done:
			return
		case task, ok := <-c.videoQueue:
			if !ok {
				return
			}

			bvid, _ := task.Detail["bvid"].(string)
			aid, _ := task.Detail["aid"].(float64)
			aidInt := int64(aid)

			progress, _ := storage.GetVideoCommentProgress(bvid)
			if c.config.Resume && progress.Done {
				fmt.Printf("[评论线程%d] %s 评论已爬完，跳过\n", threadID, bvid)
				continue
			}

			if aidInt == 0 {
				if progress.Aid != 0 {
					aidInt = progress.Aid
				} else {
					var err error
					aidInt, err = api.GetVideoAid(bvid, session, c.config.CookieConfigPath)
					if err != nil {
						fmt.Printf("[评论线程%d] 获取 %s 的aid失败: %v\n", threadID, bvid, err)
						continue
					}
					c.delay()
				}
			}

			cursor := ""
			if c.config.Resume {
				cursor = progress.Cursor
			}

			if cursor != "" {
				fmt.Printf("[评论线程%d] %s (aid=%d) 从游标 %s... 恢复爬取...\n", threadID, bvid, aidInt, truncate(cursor, 20))
			} else {
				fmt.Printf("[评论线程%d] %s (aid=%d) 开始爬取评论...\n", threadID, bvid, aidInt)
			}

			commentCount := 0
			for {
				result, err := api.GetMainComments(aidInt, cursor, session, c.config.CookieConfigPath)
				if err != nil {
					fmt.Printf("[评论线程%d] %s 评论获取错误: %v\n", threadID, bvid, err)
					storage.SaveVideoCommentProgress(bvid, cursor, aidInt)
					break
				}

				for _, reply := range result.Replies {
					rpid := fmt.Sprintf("%v", reply["rpid"])
					if mid, ok := reply["mid"]; ok {
						c.addUserMid(fmt.Sprintf("%v", mid))
					}

					if c.config.Resume && c.isRpidSaved(rpid) {
						c.stats.incCommentsSkipped()
						if rcount, ok := reply["rcount"].(float64); ok && rcount > 0 {
							c.commentQueue <- &CommentTask{Aid: aidInt, Comment: reply}
						}
						continue
					}

					if err := storage.SaveComment(reply); err == nil {
						c.stats.incCommentsSaved()
						c.markRpidSaved(rpid)
						commentCount++

						if rcount, ok := reply["rcount"].(float64); ok && rcount > 0 {
							c.commentQueue <- &CommentTask{Aid: aidInt, Comment: reply}
						}
					}
				}

				if result.IsEnd || len(result.Replies) == 0 {
					storage.MarkVideoCommentsDone(bvid)
					break
				}

				cursor = result.NextCursor
				storage.SaveVideoCommentProgress(bvid, cursor, aidInt)
				c.delay()
			}

			fmt.Printf("[评论线程%d] %s 爬取完成，共 %d 条一级评论\n", threadID, bvid, commentCount)
		}
	}
}

func (c *BiliCrawler) replyWorker(threadID int, wg *sync.WaitGroup, done <-chan struct{}, session *api.Session) {
	defer wg.Done()

	for {
		select {
		case <-done:
			return
		case task, ok := <-c.commentQueue:
			if !ok {
				return
			}

			rpid := int64(task.Comment["rpid"].(float64))
			rcount := int(task.Comment["rcount"].(float64))
			fmt.Printf("[回复线程%d] 开始爬取评论 %d 的 %d 条回复...\n", threadID, rpid, rcount)

			page := 1
			totalFetched := 0
			for {
				result, err := api.GetReplyComments(task.Aid, rpid, page, 20, session, c.config.CookieConfigPath)
				if err != nil {
					fmt.Printf("[回复线程%d] 评论 %d 回复获取错误: %v\n", threadID, rpid, err)
					break
				}

				if len(result.Replies) == 0 {
					break
				}

				for _, reply := range result.Replies {
					replyRpid := fmt.Sprintf("%v", reply["rpid"])
					if mid, ok := reply["mid"]; ok {
						c.addUserMid(fmt.Sprintf("%v", mid))
					}

					if c.config.Resume && c.isRpidSaved(replyRpid) {
						totalFetched++
						continue
					}

					if err := storage.SaveComment(reply); err == nil {
						c.stats.incRepliesSaved()
						c.markRpidSaved(replyRpid)
						totalFetched++
					}
				}

				if totalFetched >= result.TotalCount {
					break
				}
				page++
				c.delay()
			}

			fmt.Printf("[回复线程%d] 评论 %d 爬取完成，共 %d 条回复\n", threadID, rpid, totalFetched)
		}
	}
}

func (c *BiliCrawler) accountWorker(threadID int, wg *sync.WaitGroup, done <-chan struct{}, session *api.Session) {
	defer wg.Done()

	for {
		select {
		case <-done:
			return
		case mid, ok := <-c.userMidQueue:
			if !ok {
				return
			}

			if c.config.Resume && c.isMidSaved(mid) {
				c.stats.incAccountsSkipped()
				continue
			}

			userData, err := api.GetUserCard(mid, session, c.config.CookieConfigPath)
			if err != nil {
				fmt.Printf("[用户线程%d] 获取用户 %s 信息失败: %v\n", threadID, mid, err)
			} else {
				if err := storage.SaveAccount(userData); err == nil {
					c.stats.incAccountsSaved()
					c.markMidSaved(mid)
				}
			}
			c.delay()
		}
	}
}

// Run starts the crawler
func (c *BiliCrawler) Run() {
	fmt.Printf("关键词: %s\n", c.config.Keyword)
	fmt.Printf("线程数: %d\n", c.config.NThreads)
	fmt.Printf("预计搜索视频数: ~%d\n", c.config.NThreads*c.config.PagesPerThread*50)
	fmt.Printf("断点续传: %s\n", boolToStr(c.config.Resume, "启用", "禁用"))

	if c.config.Resume && len(c.videoProgress) > 0 {
		doneCount := 0
		inProgressCount := 0
		for _, p := range c.videoProgress {
			if p.Done {
				doneCount++
			} else if p.Cursor != "" {
				inProgressCount++
			}
		}
		fmt.Printf("  - 已完成评论爬取的视频: %d\n", doneCount)
		fmt.Printf("  - 评论爬取中断的视频: %d\n", inProgressCount)
	}

	// Restore pending MIDs
	if c.config.Resume && c.config.ResumePendingMids {
		pendingMids, _ := storage.GetPendingMids()
		restoredCount := 0
		for mid := range pendingMids {
			if _, saved := c.savedMids[mid]; !saved {
				c.userMids[mid] = struct{}{}
				select {
				case c.userMidQueue <- mid:
					restoredCount++
				default:
				}
			}
		}
		if restoredCount > 0 {
			fmt.Printf("  - 已恢复 %d 个待爬取的用户mid\n", restoredCount)
		}
	}

	// Start workers
	commentDone := make(chan struct{})
	replyDone := make(chan struct{})
	accountDone := make(chan struct{})

	var commentWg, replyWg, accountWg sync.WaitGroup

	// Start comment workers
	for i := 0; i < c.config.NThreads; i++ {
		commentWg.Add(1)
		session := api.NewSession(c.config.CookieConfigPath)
		go c.commentWorker(i, &commentWg, commentDone, session)
	}

	// Start reply workers
	for i := 0; i < c.config.NThreads; i++ {
		replyWg.Add(1)
		session := api.NewSession(c.config.CookieConfigPath)
		go c.replyWorker(i, &replyWg, replyDone, session)
	}

	// Start account workers
	for i := 0; i < c.config.NThreads; i++ {
		accountWg.Add(1)
		session := api.NewSession(c.config.CookieConfigPath)
		go c.accountWorker(i, &accountWg, accountDone, session)
	}

	// Search and fetch video details
	c.searchVideosParallel()

	// Wait for video queue to be processed
	close(c.videoQueue)
	commentWg.Wait()
	fmt.Printf("一级评论爬取完成，共保存 %d 条\n", c.stats.CommentsSaved)

	// Signal comment workers done, wait for reply workers
	close(commentDone)
	close(c.commentQueue)
	replyWg.Wait()
	fmt.Printf("二级评论爬取完成，共保存 %d 条\n", c.stats.RepliesSaved)

	// Signal reply workers done, wait for account workers
	close(replyDone)
	close(c.userMidQueue)
	accountWg.Wait()
	fmt.Printf("用户信息爬取完成，共保存 %d 个\n", c.stats.AccountsSaved)

	close(accountDone)

	// Print final stats
	fmt.Printf("保存视频数: %d\n", c.stats.VideosSaved)
	if c.stats.VideosSkipped > 0 {
		fmt.Printf("跳过视频数（已存在）: %d\n", c.stats.VideosSkipped)
	}
	fmt.Printf("保存一级评论数: %d\n", c.stats.CommentsSaved)
	if c.stats.CommentsSkipped > 0 {
		fmt.Printf("跳过评论数（已存在）: %d\n", c.stats.CommentsSkipped)
	}
	fmt.Printf("保存二级评论数: %d\n", c.stats.RepliesSaved)
	fmt.Printf("总评论数: %d\n", c.stats.CommentsSaved+c.stats.RepliesSaved)
	fmt.Printf("保存用户数: %d\n", c.stats.AccountsSaved)
	if c.stats.AccountsSkipped > 0 {
		fmt.Printf("跳过用户数（已存在）: %d\n", c.stats.AccountsSkipped)
	}

	// Clean up pending MIDs
	c.mu.Lock()
	remainingMids := make(map[string]struct{})
	for mid := range c.userMids {
		if _, saved := c.savedMids[mid]; !saved {
			remainingMids[mid] = struct{}{}
		}
	}
	c.mu.Unlock()

	storage.UpdatePendingMids(remainingMids)
	if len(remainingMids) > 0 {
		fmt.Printf("剩余未爬取用户数: %d\n", len(remainingMids))
	} else {
		fmt.Println("所有用户信息已爬取完成，pending_mids已清理")
	}
}

func (c *BiliCrawler) searchVideosParallel() {
	fmt.Printf("搜索视频 (关键词: %s)\n", c.config.Keyword)

	// Collect search results
	resultsChan := make(chan map[string]interface{}, c.config.NThreads*c.config.PagesPerThread*50)
	var searchWg sync.WaitGroup

	for i := 0; i < c.config.NThreads; i++ {
		searchWg.Add(1)
		session := api.NewSession(c.config.CookieConfigPath)
		go c.searchWorker(i, c.config.PagesPerThread, resultsChan, &searchWg, session)
	}

	// Wait for search to complete and close results channel
	go func() {
		searchWg.Wait()
		close(resultsChan)
	}()

	// Deduplicate results
	seenBvids := make(map[string]struct{})
	var uniqueVideos []map[string]interface{}

	for video := range resultsChan {
		bvid, ok := video["bvid"].(string)
		if !ok || bvid == "" {
			continue
		}
		if _, seen := seenBvids[bvid]; !seen {
			seenBvids[bvid] = struct{}{}
			uniqueVideos = append(uniqueVideos, video)
		}
	}

	// Filter out already saved videos in resume mode
	if c.config.Resume && len(c.savedBvids) > 0 {
		beforeCount := len(uniqueVideos)
		var newVideos []map[string]interface{}
		for _, v := range uniqueVideos {
			bvid := v["bvid"].(string)
			if _, saved := c.savedBvids[bvid]; saved {
				// Push to video queue for comment crawling
				c.videoQueue <- &VideoTask{Detail: v}
			} else {
				newVideos = append(newVideos, v)
			}
		}
		uniqueVideos = newVideos
		skipped := beforeCount - len(uniqueVideos)
		if skipped > 0 {
			c.stats.VideosSkipped = skipped
		}
	}

	fmt.Printf("共 %d 个新视频\n", len(uniqueVideos))

	if len(uniqueVideos) == 0 {
		fmt.Println("没有新视频需要获取详情")
		return
	}

	// Distribute videos to detail workers
	videoChan := make(chan map[string]interface{}, len(uniqueVideos))
	for _, v := range uniqueVideos {
		videoChan <- v
	}
	close(videoChan)

	var detailWg sync.WaitGroup
	for i := 0; i < c.config.NThreads; i++ {
		detailWg.Add(1)
		session := api.NewSession(c.config.CookieConfigPath)
		go c.videoDetailWorker(i, videoChan, &detailWg, session)
	}

	detailWg.Wait()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func boolToStr(b bool, trueStr, falseStr string) string {
	if b {
		return trueStr
	}
	return falseStr
}
