package main

import (
	_ "embed"
	"fmt"
	"log"

	toml "github.com/rokuosan/go-toml"
)

//go:embed config.toml
var configData []byte

type Config struct {
	Title    string    `toml:"title"`
	Enabled  bool      `toml:"enabled"`
	Ports    []int     `toml:"ports"`
	Database Database  `toml:"database"`
	Services []Service `toml:"services"`
}

type Database struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
}

type Service struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

func main() {
	var cfg Config
	if err := toml.Unmarshal(configData, &cfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("title: %s\n", cfg.Title)
	fmt.Printf("enabled: %t\n", cfg.Enabled)
	fmt.Printf("ports: %v\n", cfg.Ports)
	fmt.Printf("database: %s:%d (%s)\n", cfg.Database.Host, cfg.Database.Port, cfg.Database.User)

	for _, service := range cfg.Services {
		fmt.Printf("service: %s -> %s\n", service.Name, service.URL)
	}
}
