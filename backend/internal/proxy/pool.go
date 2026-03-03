package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultConfig is the proxy pool configuration used across all scrapers.
var DefaultConfig = Config{
	Host:        "dc.decodo.com",
	Username:    "spawee4ylf",
	Password:    "yIzsp7~aeb7Yrz87RQ",
	PortStart:   10001,
	PortEnd:     10100,
	MinInterval: 3.8,
}

// Config holds proxy pool settings.
type Config struct {
	Host        string
	Username    string
	Password    string
	PortStart   int
	PortEnd     int
	MinInterval float64 // seconds between requests per proxy
}

// Worker manages rate limiting for a single proxy.
type Worker struct {
	ProxyURL    string
	Client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

// NewWorker creates a rate-limited proxy worker.
func NewWorker(proxyURL string, minInterval time.Duration) *Worker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 10,
	}
	return &Worker{
		ProxyURL:    proxyURL,
		Client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: minInterval,
	}
}

// Do executes an HTTP request with rate limiting.
func (w *Worker) Do(req *http.Request) (*http.Response, error) {
	w.mu.Lock()
	elapsed := time.Since(w.lastUsed)
	if elapsed < w.minInterval {
		time.Sleep(w.minInterval - elapsed)
	}
	w.lastUsed = time.Now()
	w.mu.Unlock()
	return w.Client.Do(req)
}

// Get executes an HTTP GET with rate limiting.
func (w *Worker) Get(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	return w.Do(req)
}

// Pool holds a set of proxy workers and distributes work across them.
type Pool struct {
	Workers []*Worker
	mu      sync.Mutex
	next    int
}

// NewPool creates a proxy pool from config.
func NewPool(cfg Config) *Pool {
	interval := time.Duration(cfg.MinInterval * float64(time.Second))
	urls := BuildURLs(cfg)
	workers := make([]*Worker, len(urls))
	for i, u := range urls {
		workers[i] = NewWorker(u, interval)
	}
	return &Pool{Workers: workers}
}

// NextWorker returns the next available worker (round-robin).
func (p *Pool) NextWorker() *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()
	w := p.Workers[p.next]
	p.next = (p.next + 1) % len(p.Workers)
	return w
}

// Size returns the number of workers in the pool.
func (p *Pool) Size() int {
	return len(p.Workers)
}

// BuildURLs generates proxy URLs from config.
func BuildURLs(cfg Config) []string {
	var urls []string
	for port := cfg.PortStart; port <= cfg.PortEnd; port++ {
		urls = append(urls, fmt.Sprintf("http://%s:%s@%s:%d",
			cfg.Username, cfg.Password, cfg.Host, port))
	}
	return urls
}

// BuildURLsFromExample parses "host:port:user:pass" format and generates pool URLs.
func BuildURLsFromExample(example string, portStart, portEnd int) []string {
	parts := strings.Split(example, ":")
	if len(parts) < 4 {
		panic("Proxy format must be host:port:username:password")
	}
	cfg := Config{
		Host:      parts[0],
		Username:  parts[2],
		Password:  parts[3],
		PortStart: portStart,
		PortEnd:   portEnd,
	}
	return BuildURLs(cfg)
}
