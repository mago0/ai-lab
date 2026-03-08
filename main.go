package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/mattw/ai-lab/internal/config"
)

func main() {
	_ = godotenv.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	fmt.Fprintf(os.Stderr, "ai-lab starting on %s:%d\n", cfg.DashboardHost, cfg.DashboardPort)
}
