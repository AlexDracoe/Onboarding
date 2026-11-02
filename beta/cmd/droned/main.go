package main

import (
	"encoding/json"
	"fmt"
	"log"

	"beta/internal/config"
	"beta/internal/domain"
)

func floatPtr(v float64) *float64 {
	return &v
}

func main() {
	cfg := config.Load()
	fmt.Printf("config: %+v\n", cfg)

	var meow domain.VehicleState

	jsonData, err := json.Marshal(meow)
	if err != nil {
		log.Fatalf("Error marshaling to JSON: %s", err)
	}
	fmt.Println(string(jsonData))

	if meow.Heading != nil {
		fmt.Println("Heading ", *meow.Heading)
	} else {
		fmt.Println("Heading not set")
	}

	meow.Heading = new(2.1)

	if meow.Heading != nil {
		fmt.Println("Heading", *meow.Heading)
	} else {
		fmt.Println("Heading not set")
	}

	var vel domain.Velocity
	vel.VelocityX = 12.2
	vel.VelocityY = 143.2
	vel.VelocityZ = 4.3

	meow.Velocity = &vel

	if meow.Velocity != nil {
		fmt.Println("Velocity", *meow.Velocity)
	}

	meow.Position = &domain.Position{Latitude: 12.2, Longitude: 3.0, Altitude: 24.1, RelativeAltitude: 91.0}

	if meow.Position != nil {
		fmt.Println("Position", *meow.Position)
	}
}
