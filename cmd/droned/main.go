package main

import (
	"fmt"

	"beta/internal/config"
)

func main() {
	cfg := config.Load()
	fmt.Printf("config: %+v\n", cfg)

}
