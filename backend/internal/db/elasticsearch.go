package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

// NewElasticsearchClient creates an Elasticsearch client from environment variables.
// It retries the connection up to 10 times with exponential backoff (max ~60s total wait).
func NewElasticsearchClient() (*elasticsearch.Client, error) {
	node := os.Getenv("ELASTICSEARCH_NODE")
	if node == "" {
		node = "http://localhost:9200"
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:  []string{node},
		MaxRetries: 3,
		Transport:  nil, // use default
	})
	if err != nil {
		return nil, fmt.Errorf("create elasticsearch client: %w", err)
	}

	const maxAttempts = 10
	backoff := 1 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = pingES(client)
		if err == nil {
			log.Printf("elasticsearch: connected to %s", node)
			return client, nil
		}
		if attempt == maxAttempts {
			break
		}
		log.Printf("elasticsearch: attempt %d/%d failed (%v), retrying in %v…", attempt, maxAttempts, err, backoff)
		time.Sleep(backoff)
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("elasticsearch: gave up after %d attempts: %w", maxAttempts, err)
}

func pingES(client *elasticsearch.Client) error {
	done := make(chan error, 1)
	go func() {
		res, err := client.Info()
		if err != nil {
			done <- fmt.Errorf("elasticsearch info: %w", err)
			return
		}
		res.Body.Close()
		if res.IsError() {
			done <- fmt.Errorf("elasticsearch info: status %s", res.Status())
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("elasticsearch ping timeout")
	}
}
