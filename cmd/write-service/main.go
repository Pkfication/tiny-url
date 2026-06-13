package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"go.elastic.co/apm/module/apmhttp/v2"
)

type Config struct {
	Port      string
	KGSURL    string
	RedisAddr string
}

func loadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	kgsURL := os.Getenv("KGS_URL")
	if kgsURL == "" {
		kgsURL = "http://localhost:8080/key"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	return &Config{
		Port:      port,
		KGSURL:    kgsURL,
		RedisAddr: redisAddr,
	}
}

type Service struct {
	cfg *Config
	rdb *redis.Client
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	TinyURL string `json:"tiny_url"`
}

func (s *Service) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// 1. Call KGS to get a unique key
	key, err := s.fetchKey(r.Context())
	if err != nil {
		log.Printf("Failed to fetch key from KGS: %v", err)
		http.Error(w, "Failed to generate short key", http.StatusInternalServerError)
		return
	}

	// 2. Save mapping in Redis
	err = s.rdb.Set(r.Context(), key, req.URL, 0).Err()
	if err != nil {
		log.Printf("Failed to save mapping to Redis: %v", err)
		http.Error(w, "Failed to save URL mapping", http.StatusInternalServerError)
		return
	}

	// 3. Return shortened URL
	tinyURL := fmt.Sprintf("http://localhost:8080/%s", key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ShortenResponse{
		TinyURL: tinyURL,
	})
}

func (s *Service) fetchKey(ctx context.Context) (string, error) {
	client := apmhttp.WrapClient(http.DefaultClient)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.KGSURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("KGS returned status: %d", resp.StatusCode)
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Key == "" {
		return "", fmt.Errorf("KGS returned empty key")
	}

	return result.Key, nil
}

func main() {
	cfg := loadConfig()

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	svc := &Service{
		cfg: cfg,
		rdb: rdb,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/shorten", svc.handleShorten)

	tracedHandler := apmhttp.Wrap(mux)

	log.Printf("Write service listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, tracedHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
