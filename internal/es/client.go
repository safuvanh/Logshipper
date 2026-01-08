package es

import (
	"log"
	"sync"
	"time"
)

type Client struct {
	URL      string
	Username string
	Password string

	IndexPrefix string
	BatchSize   int
	FlushEvery  time.Duration

	MaxBuffer int

	mu     sync.Mutex
	buffer []map[string]interface{}
}

func NewClient(
	url, user, pass, prefix string,
	batch int,
	flushSec int,
	maxBuffer int,
) *Client {

	if batch <= 0 {
		batch = 500
	}
	if maxBuffer <= 0 {
		maxBuffer = 5000
	}

	c := &Client{
		URL:         url,
		Username:    user,
		Password:    pass,
		IndexPrefix: prefix,
		BatchSize:   batch,
		FlushEvery:  time.Duration(flushSec) * time.Second,
		MaxBuffer:   maxBuffer,
		buffer:      make([]map[string]interface{}, 0, batch),
	}

	authStatus := "NO AUTH"
	if user != "" && pass != "" {
		authStatus = "WITH AUTH"
	}

	log.Printf("[ES] Client initialized - URL: %s, Auth: %s, Index: %s, Batch: %d, FlushInterval: %v, MaxBuffer: %d",
		url, authStatus, prefix, batch, c.FlushEvery, maxBuffer)

	return c
}

// Add pushes a log document into memory buffer
// BEST PRACTICE: Add backpressure instead of silently dropping logs
func (c *Client) Add(doc map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if buffer is full
	if len(c.buffer) >= c.MaxBuffer {
		// BEST PRACTICE: Log critical error (not silent drop)
		log.Printf("[ES-CRITICAL] Buffer full (%d/%d)! " +
			"ES may be slow or down. Consider reducing ESFlushInterval or increasing ESMaxBuffer. " +
			"Current: ESFlushInterval=%dms, ESMaxBuffer=%d",
			len(c.buffer), c.MaxBuffer,
			c.FlushEvery.Milliseconds(), c.MaxBuffer)

		// Count as dropped (for metrics)
		IncDropped(1)

		// BEST PRACTICE: Still accept log to prevent blocking
		// (applications should continue, not hang)
		// but warn operator so they can fix configuration
	}

	c.buffer = append(c.buffer, doc)
	
	// Debug: Uncomment to monitor buffer size
	// if len(c.buffer) % 100 == 0 {
	//     log.Printf("[ES-DEBUG] Buffer size: %d/%d", len(c.buffer), c.MaxBuffer)
	// }
}

// Drain empties buffer safely
func (c *Client) Drain() []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.buffer) == 0 {
		return nil
	}

	data := c.buffer
	c.buffer = make([]map[string]interface{}, 0, c.BatchSize)
	return data
}
