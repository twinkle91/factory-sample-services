package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// config is read from the environment. DATABASE_URL and REDIS_URL are required;
// PORT defaults to 8080. The server always listens on 0.0.0.0.
type config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
}

func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		DatabaseURL: getenv("DATABASE_URL"),
		RedisURL:    getenv("REDIS_URL"),
		Port:        getenv("PORT"),
	}
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL (for example postgres://factory:factory@postgres:5432/app?sslmode=disable)")
	}
	if c.RedisURL == "" {
		missing = append(missing, "REDIS_URL (for example redis://redis:6379/0)")
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, "; "))
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return config{}, fmt.Errorf("PORT must be a number from 1 to 65535, got %q", c.Port)
	}
	return c, nil
}

func (c config) addr() string {
	return net.JoinHostPort("0.0.0.0", c.Port)
}
