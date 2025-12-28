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
func (c *Client) Add(doc map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.buffer) >= c.MaxBuffer {
		drop := c.MaxBuffer / 10
		if drop < 1 {
			drop = 1
		}

		c.buffer = c.buffer[drop:]
		IncDropped(drop)

		log.Printf("[ES-WARN] Buffer overflow! Dropped %d oldest logs. Max capacity: %d", drop, c.MaxBuffer)
	}

	c.buffer = append(c.buffer, doc)
	
	// Minimal logging - only warn on buffer overflow
	// Removed: frequent buffer size logging to reduce noise
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
