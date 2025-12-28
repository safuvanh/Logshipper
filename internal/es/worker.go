package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func (c *Client) StartWorker() {
	ticker := time.NewTicker(c.FlushEvery)

	log.Printf("[ES] Worker started - flushes every %v", c.FlushEvery)

	go func() {
		for range ticker.C {
			c.flush()
		}
	}()
}

func (c *Client) flush() {
	docs := c.Drain()
	if len(docs) == 0 {
		return
	}

	index := fmt.Sprintf("%s-%s",
		c.IndexPrefix,
		time.Now().Format("2006.01.02"),
	)

	batchNum := 0
	for i := 0; i < len(docs); i += c.BatchSize {
		batchNum++
		end := i + c.BatchSize
		if end > len(docs) {
			end = len(docs)
		}

		batchSize := end - i

		var buf bytes.Buffer
		for _, doc := range docs[i:end] {
			meta := fmt.Sprintf(`{ "index": { "_index": "%s" } }%s`, index, "\n")
			data, _ := json.Marshal(doc)
			buf.WriteString(meta)
			buf.Write(data)
			buf.WriteByte('\n')
		}

		// Build ES bulk URL, handling trailing slashes properly
		esURL := strings.TrimSuffix(c.URL, "/") + "/_bulk"

		req, err := http.NewRequest("POST", esURL, &buf)
		if err != nil {
			log.Printf("[ES-ERROR] FAILED batch #%d: request creation error: %v", batchNum, err)
			IncBulkError()
			continue
		}

		// Set basic auth ONLY if both username and password are provided
		if c.Username != "" && c.Password != "" {
			req.SetBasicAuth(c.Username, c.Password)
		}
		req.Header.Set("Content-Type", "application/x-ndjson")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req = req.WithContext(ctx)

		startTime := time.Now()
		resp, err := http.DefaultClient.Do(req)
		elapsed := time.Since(startTime)
		cancel()

		if err != nil {
			log.Printf("[ES-ERROR] FAILED batch #%d: network error after %v: %v", batchNum, elapsed, err)
			IncBulkError()
			continue
		}

		statusCode := resp.StatusCode
		resp.Body.Close()

		if statusCode >= 300 {
			log.Printf("[ES-ERROR] FAILED batch #%d: HTTP %d response (after %v)", batchNum, statusCode, elapsed)
			IncBulkError()
		} else {
			IncSent(batchSize)
			log.Printf("[ES-SUCCESS] Batch #%d: %d docs sent in %v | Index: %s", batchNum, batchSize, elapsed, index)
		}
	}
}
