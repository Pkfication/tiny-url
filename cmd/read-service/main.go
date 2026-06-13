package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.elastic.co/apm/module/apmhttp/v2"
)

type Config struct {
	Port      string
	RedisAddr string
}

func loadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	return &Config{
		Port:      port,
		RedisAddr: redisAddr,
	}
}

type Service struct {
	cfg *Config
	rdb *redis.Client
}

func (s *Service) handleRedirect(w http.ResponseWriter, r *http.Request) {
	// Extract the key from the request path (e.g., "/8M3w1" -> "8M3w1")
	key := strings.TrimPrefix(r.URL.Path, "/")

	// If the path is empty (just "/"), return a default message
	if key == "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome to Distributed TinyURL Service! Use POST /shorten to create short links."))
		return
	}

	// Retrieve the original URL from Redis
	longURL, err := s.rdb.Get(r.Context(), key).Result()
	if err == redis.Nil {
		http.Error(w, "TinyURL not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Failed to get key from Redis: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Perform redirect
	http.Redirect(w, r, longURL, http.StatusFound)
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
	mux.HandleFunc("/", svc.handleRedirect)

	tracedHandler := apmhttp.Wrap(mux)

	log.Printf("Read service listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, tracedHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
