package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"spider-go/cookie"
	"spider-go/ratelimit"
)

const (
	defaultReferer = "https://www.bilibili.com"
)

var (
	userAgent   = "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0"
	userAgentMu sync.RWMutex
)

// SetUserAgent sets the global User-Agent for all API requests
func SetUserAgent(ua string) {
	userAgentMu.Lock()
	defer userAgentMu.Unlock()
	userAgent = ua
}

// GetUserAgent returns the current User-Agent
func GetUserAgent() string {
	userAgentMu.RLock()
	defer userAgentMu.RUnlock()
	return userAgent
}

func getDefaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent": GetUserAgent(),
		"Accept":     "application/json, text/plain, */*",
		"Referer":    defaultReferer,
	}
}

// WBI signature encoding table
var wbiMixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

var (
	wbiMixinKey       string
	wbiKeyExpireTime  time.Time
	wbiKeyMu          sync.Mutex
	wbiKeyCacheSeconds = 3600
)

// Session wraps an HTTP client with cookie management
type Session struct {
	client        *http.Client
	currentCookie string
	headers       map[string]string
}

// NewSession creates a new session with a cookie from the pool
func NewSession(cookieConfigPath string) *Session {
	pool := cookie.GetCookiePool(cookieConfigPath)
	cookieValue := pool.GetCookie()

	headers := make(map[string]string)
	for k, v := range getDefaultHeaders() {
		headers[k] = v
	}
	headers["Cookie"] = cookieValue

	session := &Session{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		currentCookie: cookieValue,
		headers:       headers,
	}

	// Initialize session by visiting bilibili.com
	req, _ := http.NewRequest("GET", "https://www.bilibili.com/", nil)
	for k, v := range session.headers {
		req.Header.Set(k, v)
	}
	session.client.Do(req)

	return session
}

// doRequest performs an HTTP request with the session's headers
func (s *Session) doRequest(method, urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	return s.client.Do(req)
}

// handleCookieError marks the current cookie as invalid if needed
func (s *Session) handleCookieError(code int, cookieConfigPath string) {
	if cookie.IsCookieError(code) && s.currentCookie != "" {
		pool := cookie.GetCookiePool(cookieConfigPath)
		pool.MarkInvalid(s.currentCookie, false)
	}
}

// md5Hash computes MD5 hash of a string
func md5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// getMixinKey generates the mixin key from img_key and sub_key
func getMixinKey(orig string) string {
	var result strings.Builder
	for _, i := range wbiMixinKeyEncTab {
		if i < len(orig) {
			result.WriteByte(orig[i])
		}
	}
	if result.Len() > 32 {
		return result.String()[:32]
	}
	return result.String()
}

// getWbiKeys fetches img_key and sub_key from the nav API
func getWbiKeys(session *Session) (string, string, error) {
	urlStr := "https://api.bilibili.com/x/web-interface/nav"

	var resp *http.Response
	var err error

	if session != nil {
		resp, err = session.doRequest("GET", urlStr)
	} else {
		req, _ := http.NewRequest("GET", urlStr, nil)
		for k, v := range getDefaultHeaders() {
			req.Header.Set(k, v)
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err = client.Do(req)
	}

	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var data struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", "", err
	}

	// Extract key from URL
	imgURL := data.Data.WbiImg.ImgURL
	subURL := data.Data.WbiImg.SubURL

	if imgURL == "" || subURL == "" {
		return "", "", fmt.Errorf("wbi_img not found in response")
	}

	// Extract filename without extension
	imgParts := strings.Split(imgURL, "/")
	imgKey := strings.Split(imgParts[len(imgParts)-1], ".")[0]

	subParts := strings.Split(subURL, "/")
	subKey := strings.Split(subParts[len(subParts)-1], ".")[0]

	return imgKey, subKey, nil
}

// GetWbiMixinKey returns the cached or freshly fetched WBI mixin key
func GetWbiMixinKey(session *Session) string {
	wbiKeyMu.Lock()
	defer wbiKeyMu.Unlock()

	if wbiMixinKey != "" && time.Now().Before(wbiKeyExpireTime) {
		return wbiMixinKey
	}

	imgKey, subKey, err := getWbiKeys(session)
	if err == nil && imgKey != "" && subKey != "" {
		wbiMixinKey = getMixinKey(imgKey + subKey)
		wbiKeyExpireTime = time.Now().Add(time.Duration(wbiKeyCacheSeconds) * time.Second)
		return wbiMixinKey
	}

	// Fallback value
	return "ea1db124af3c7062474693fa704f4ff8"
}

// GenerateWbiSign generates the WBI signature for the given parameters
func GenerateWbiSign(params map[string]string, session *Session) (string, int64) {
	mixinKey := GetWbiMixinKey(session)
	wts := time.Now().Unix()

	// Add wts to params
	paramsCopy := make(map[string]string)
	for k, v := range params {
		paramsCopy[k] = v
	}
	paramsCopy["wts"] = fmt.Sprintf("%d", wts)

	// Sort keys
	keys := make([]string, 0, len(paramsCopy))
	for k := range paramsCopy {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, paramsCopy[k]))
	}
	queryString := strings.Join(parts, "&")

	// Generate signature
	signString := queryString + mixinKey
	wRid := md5Hash(signString)

	return wRid, wts
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries int
	BaseDelay  float64
	MaxDelay   float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1.0,
		MaxDelay:   30.0,
	}
}

// withRetry wraps a function with retry logic
func withRetry[T any](fn func() (T, error), config RetryConfig) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		ratelimit.WaitForToken()

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err
		if attempt < config.MaxRetries {
			delay := config.BaseDelay * float64(int(1)<<attempt)
			delay += rand.Float64()
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			time.Sleep(time.Duration(delay * float64(time.Second)))
		}
	}

	return zero, lastErr
}

// SearchResult represents a video search result
type SearchResult struct {
	Videos   []map[string]interface{}
	NumPages int
}

// SearchVideos searches for videos by keyword
func SearchVideos(keyword string, page, pageSize int, session *Session, cookieConfigPath string) (*SearchResult, error) {
	return withRetry(func() (*SearchResult, error) {
		urlStr := fmt.Sprintf("https://api.bilibili.com/x/web-interface/search/type?page=%d&page_size=%d&keyword=%s&search_type=video&order=",
			page, pageSize, url.QueryEscape(keyword))

		var resp *http.Response
		var err error

		if session != nil {
			resp, err = session.doRequest("GET", urlStr)
		} else {
			req, _ := http.NewRequest("GET", urlStr, nil)
			for k, v := range getDefaultHeaders() {
				req.Header.Set(k, v)
			}
			client := &http.Client{Timeout: 15 * time.Second}
			resp, err = client.Do(req)
		}

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Result   []map[string]interface{} `json:"result"`
				NumPages int                      `json:"numPages"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		if data.Code != 0 {
			if session != nil {
				session.handleCookieError(data.Code, cookieConfigPath)
			}
			return nil, fmt.Errorf("%s", data.Message)
		}

		return &SearchResult{
			Videos:   data.Data.Result,
			NumPages: data.Data.NumPages,
		}, nil
	}, DefaultRetryConfig())
}

// GetVideoDetail fetches video details by BVID
func GetVideoDetail(bvid string, session *Session, cookieConfigPath string) (map[string]interface{}, error) {
	return withRetry(func() (map[string]interface{}, error) {
		urlStr := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)

		var resp *http.Response
		var err error

		if session != nil {
			resp, err = session.doRequest("GET", urlStr)
		} else {
			req, _ := http.NewRequest("GET", urlStr, nil)
			for k, v := range getDefaultHeaders() {
				req.Header.Set(k, v)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err = client.Do(req)
		}

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var data struct {
			Code    int                    `json:"code"`
			Message string                 `json:"message"`
			Data    map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		if data.Code != 0 {
			if session != nil {
				session.handleCookieError(data.Code, cookieConfigPath)
			}
			return nil, fmt.Errorf("%s", data.Message)
		}

		return data.Data, nil
	}, DefaultRetryConfig())
}

// GetVideoAid fetches the AID for a video by BVID
func GetVideoAid(bvid string, session *Session, cookieConfigPath string) (int64, error) {
	detail, err := GetVideoDetail(bvid, session, cookieConfigPath)
	if err != nil {
		return 0, err
	}

	aid, ok := detail["aid"].(float64)
	if !ok {
		return 0, fmt.Errorf("aid not found in response")
	}

	return int64(aid), nil
}

// MainCommentsResult represents the result of fetching main comments
type MainCommentsResult struct {
	Replies    []map[string]interface{}
	NextCursor string
	IsEnd      bool
}

// GetMainComments fetches main comments for a video
func GetMainComments(oid int64, cursor string, session *Session, cookieConfigPath string) (*MainCommentsResult, error) {
	return withRetry(func() (*MainCommentsResult, error) {
		var paginationStr string
		if cursor != "" {
			paginationStr = fmt.Sprintf(`{"offset":"%s"}`, cursor)
		} else {
			paginationStr = `{"offset":""}`
		}

		paginationStrEncoded := url.QueryEscape(paginationStr)

		mode := 2
		plat := 1
		typeVal := 1
		webLocation := 1315875

		mixinKey := GetWbiMixinKey(session)
		wts := time.Now().Unix()

		var signStr string
		if cursor != "" {
			signStr = fmt.Sprintf("mode=%d&oid=%d&pagination_str=%s&plat=%d&type=%d&web_location=%d&wts=%d",
				mode, oid, paginationStrEncoded, plat, typeVal, webLocation, wts)
		} else {
			signStr = fmt.Sprintf("mode=%d&oid=%d&pagination_str=%s&plat=%d&seek_rpid=&type=%d&web_location=%d&wts=%d",
				mode, oid, paginationStrEncoded, plat, typeVal, webLocation, wts)
		}

		wRid := md5Hash(signStr + mixinKey)

		// Build URL with custom encoding for pagination_str
		paginationStrForURL := url.QueryEscape(paginationStr)
		// Replace %3A back to : for the URL (safe character)
		paginationStrForURL = strings.ReplaceAll(paginationStrForURL, "%3A", ":")

		var urlStr string
		if cursor != "" {
			urlStr = fmt.Sprintf("https://api.bilibili.com/x/v2/reply/wbi/main?oid=%d&type=%d&mode=%d&pagination_str=%s&plat=%d&web_location=%d&w_rid=%s&wts=%d",
				oid, typeVal, mode, paginationStrForURL, plat, webLocation, wRid, wts)
		} else {
			urlStr = fmt.Sprintf("https://api.bilibili.com/x/v2/reply/wbi/main?oid=%d&type=%d&mode=%d&pagination_str=%s&plat=%d&seek_rpid=&web_location=%d&w_rid=%s&wts=%d",
				oid, typeVal, mode, paginationStrForURL, plat, webLocation, wRid, wts)
		}

		var resp *http.Response
		var err error

		if session != nil {
			resp, err = session.doRequest("GET", urlStr)
		} else {
			req, _ := http.NewRequest("GET", urlStr, nil)
			for k, v := range getDefaultHeaders() {
				req.Header.Set(k, v)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err = client.Do(req)
		}

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Replies []map[string]interface{} `json:"replies"`
				Cursor  struct {
					IsEnd           bool `json:"is_end"`
					PaginationReply struct {
						NextOffset string `json:"next_offset"`
					} `json:"pagination_reply"`
				} `json:"cursor"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		if data.Code != 0 {
			if session != nil {
				session.handleCookieError(data.Code, cookieConfigPath)
			}
			return nil, fmt.Errorf("%s", data.Message)
		}

		replies := data.Data.Replies
		if replies == nil {
			replies = []map[string]interface{}{}
		}

		nextCursor := data.Data.Cursor.PaginationReply.NextOffset
		isEnd := data.Data.Cursor.IsEnd

		if nextCursor == "" {
			isEnd = true
		}

		return &MainCommentsResult{
			Replies:    replies,
			NextCursor: nextCursor,
			IsEnd:      isEnd,
		}, nil
	}, DefaultRetryConfig())
}

// ReplyCommentsResult represents the result of fetching reply comments
type ReplyCommentsResult struct {
	Replies    []map[string]interface{}
	TotalCount int
}

// GetReplyComments fetches reply comments for a parent comment
func GetReplyComments(oid int64, rootRpid int64, page, pageSize int, session *Session, cookieConfigPath string) (*ReplyCommentsResult, error) {
	return withRetry(func() (*ReplyCommentsResult, error) {
		urlStr := fmt.Sprintf("https://api.bilibili.com/x/v2/reply/reply?oid=%d&type=1&root=%d&ps=%d&pn=%d",
			oid, rootRpid, pageSize, page)

		var resp *http.Response
		var err error

		if session != nil {
			resp, err = session.doRequest("GET", urlStr)
		} else {
			req, _ := http.NewRequest("GET", urlStr, nil)
			for k, v := range getDefaultHeaders() {
				req.Header.Set(k, v)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err = client.Do(req)
		}

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Replies []map[string]interface{} `json:"replies"`
				Page    struct {
					Count int `json:"count"`
				} `json:"page"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		if data.Code != 0 {
			if session != nil {
				session.handleCookieError(data.Code, cookieConfigPath)
			}
			return nil, fmt.Errorf("%s", data.Message)
		}

		replies := data.Data.Replies
		if replies == nil {
			replies = []map[string]interface{}{}
		}

		return &ReplyCommentsResult{
			Replies:    replies,
			TotalCount: data.Data.Page.Count,
		}, nil
	}, DefaultRetryConfig())
}

// GetUserCard fetches user card information
func GetUserCard(mid string, session *Session, cookieConfigPath string) (map[string]interface{}, error) {
	return withRetry(func() (map[string]interface{}, error) {
		urlStr := fmt.Sprintf("https://api.bilibili.com/x/web-interface/card?mid=%s&photo=true", mid)

		var resp *http.Response
		var err error

		if session != nil {
			resp, err = session.doRequest("GET", urlStr)
		} else {
			req, _ := http.NewRequest("GET", urlStr, nil)
			for k, v := range getDefaultHeaders() {
				req.Header.Set(k, v)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err = client.Do(req)
		}

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var data struct {
			Code    int                    `json:"code"`
			Message string                 `json:"message"`
			Data    map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		if data.Code != 0 {
			if session != nil {
				session.handleCookieError(data.Code, cookieConfigPath)
			}
			return nil, fmt.Errorf("%s", data.Message)
		}

		return data.Data, nil
	}, DefaultRetryConfig())
}
