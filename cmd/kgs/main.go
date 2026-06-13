package main

import (
	"log"
	"net/http"

	"kgs/internal/api"
	"kgs/internal/config"
	"kgs/internal/core"
	"kgs/internal/zookeeper"

	"go.elastic.co/apm/module/apmhttp/v2"
)

func main() {
	cfg := config.Load()

	// 1. Initialize ZooKeeper Client
	zkClient, err := zookeeper.NewClient(cfg.ZKServers)
	if err != nil {
		log.Fatalf("Failed to initialize ZooKeeper client: %v", err)
	}
	defer zkClient.Close()

	// 2. Initialize Core Service with ZooKeeper Client as RangeProvider
	keyService, err := core.NewKeyService(zkClient)
	if err != nil {
		log.Fatalf("Failed to initialize KeyService: %v", err)
	}

	// 3. Initialize API Handler
	handler := api.NewHandler(keyService)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Wrap the mux with APM middleware
	tracedHandler := apmhttp.Wrap(mux)

	// 4. Start Server
	log.Printf("KGS server listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, tracedHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
