package cookie

import (
	"encoding/json"
	"math/rand"
	"os"
	"sync"
)

// CookieItem represents a single cookie with its metadata
type CookieItem struct {
	Value     string `json:"value"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	IsValid   bool   `json:"-"`
	FailCount int    `json:"-"`
	MaxFails  int    `json:"-"`
}

// MarkFailed increments the fail count and returns true if the cookie should be disabled
func (c *CookieItem) MarkFailed() bool {
	c.FailCount++
	if c.FailCount >= c.MaxFails {
		c.IsValid = false
		return true
	}
	return false
}

// Reset resets the fail count and marks the cookie as valid
func (c *CookieItem) Reset() {
	c.FailCount = 0
	c.IsValid = true
}

// CookieConfig represents the JSON configuration file structure
type CookieConfig struct {
	Cookies  []CookieItem   `json:"cookies"`
	Settings CookieSettings `json:"settings"`
}

// CookieSettings represents the settings section of the config
type CookieSettings struct {
	Strategy       string `json:"strategy"`
	ValidateOnLoad bool   `json:"validate_on_load"`
}

// CookiePool manages a pool of cookies with rotation strategies
type CookiePool struct {
	cookies    []*CookieItem
	mu         sync.RWMutex
	index      int
	strategy   string
	configPath string
}

// NewCookiePool creates a new cookie pool from the given config file
func NewCookiePool(configPath string) *CookiePool {
	pool := &CookiePool{
		cookies:    make([]*CookieItem, 0),
		strategy:   "round_robin",
		configPath: configPath,
	}
	pool.loadCookies()
	return pool
}

// loadCookies loads cookies from the configuration file
func (p *CookiePool) loadCookies() {
	data, err := os.ReadFile(p.configPath)
	if err != nil {
		return
	}

	var config CookieConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	if config.Settings.Strategy != "" {
		p.strategy = config.Settings.Strategy
	}

	for i := range config.Cookies {
		cookie := &config.Cookies[i]
		if cookie.Enabled && cookie.Value != "" {
			cookie.IsValid = true
			cookie.MaxFails = 3
			p.cookies = append(p.cookies, cookie)
		}
	}
}

// GetCookie returns a cookie value based on the rotation strategy
func (p *CookiePool) GetCookie() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	available := p.getAvailable()
	if len(available) == 0 {
		return ""
	}

	var cookie *CookieItem
	if p.strategy == "random" {
		cookie = available[rand.Intn(len(available))]
	} else { // round_robin
		p.index = p.index % len(available)
		cookie = available[p.index]
		p.index++
	}

	return cookie.Value
}

// GetCookieItem returns a cookie item based on the rotation strategy
func (p *CookiePool) GetCookieItem() *CookieItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	available := p.getAvailable()
	if len(available) == 0 {
		return nil
	}

	if p.strategy == "random" {
		return available[rand.Intn(len(available))]
	}

	// round_robin
	p.index = p.index % len(available)
	cookie := available[p.index]
	p.index++
	return cookie
}

// getAvailable returns all available (enabled and valid) cookies
func (p *CookiePool) getAvailable() []*CookieItem {
	available := make([]*CookieItem, 0)
	for _, c := range p.cookies {
		if c.Enabled && c.IsValid {
			available = append(available, c)
		}
	}
	return available
}

// MarkInvalid marks a cookie as invalid by its value
func (p *CookiePool) MarkInvalid(cookieValue string, permanent bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, cookie := range p.cookies {
		if cookie.Value == cookieValue {
			if permanent {
				cookie.IsValid = false
				cookie.Enabled = false
			} else {
				cookie.MarkFailed()
			}
			break
		}
	}
}

// GetStatus returns the current status of the cookie pool
func (p *CookiePool) GetStatus() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := len(p.cookies)
	enabled := 0
	valid := 0
	for _, c := range p.cookies {
		if c.Enabled {
			enabled++
			if c.IsValid {
				valid++
			}
		}
	}

	return map[string]interface{}{
		"total":    total,
		"enabled":  enabled,
		"valid":    valid,
		"strategy": p.strategy,
	}
}

// Len returns the number of available cookies
func (p *CookiePool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.getAvailable())
}

var (
	globalPool *CookiePool
	poolOnce   sync.Once
)

// GetCookiePool returns the global cookie pool singleton
func GetCookiePool(configPath string) *CookiePool {
	poolOnce.Do(func() {
		globalPool = NewCookiePool(configPath)
	})
	return globalPool
}

// IsCookieError checks if the error code indicates a cookie-related error
func IsCookieError(code int) bool {
	// -101: Not logged in
	// -352: Risk control check failed
	// -412: Request blocked
	return code == -101 || code == -352 || code == -412
}
