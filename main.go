package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	bucket := envOr("MEDIA_BUCKET", "media-stream-assets")
	storage := NewInfraiStorage(key)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := storage.CreateBucket(ctx, bucket); err != nil {
		log.Fatalf("event=bucket_setup_failed bucket=%q error=%q", bucket, err)
	}

	service := &MediaService{pipeline: NewAssetPipeline(storage, bucket)}
	addr := envOr("LISTEN_ADDR", ":8080")
	log.Printf("event=service_ready addr=%q bucket=%q", addr, bucket)
	server := &http.Server{Addr: addr, Handler: service.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
