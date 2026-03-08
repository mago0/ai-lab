package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Fprintf(os.Stderr, "ai-lab starting...\n")
}
