package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Addr     string
	MongoURI string
	Database string
	DemoMode bool
	SeedDemo bool
	WebDir   string
}

// loadEnvFile reads backend/.env without overriding variables already set by the shell.
// Values are read literally after the first '='; passwords may contain #, = and spaces.
func loadEnvFile() error {
	path := ".env"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		path = filepath.Join("backend", ".env")
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid line in %s: expected KEY=value", path)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if key == "" {
			return fmt.Errorf("empty key in %s", path)
		}
		if _, set := os.LookupEnv(key); !set {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func readConfig() (config, error) {
	cfg := config{Addr: os.Getenv("ADDR"), Database: os.Getenv("MONGODB_DATABASE"), DemoMode: os.Getenv("DEMO_MODE") == "true", SeedDemo: os.Getenv("SEED_DEMO") == "true", WebDir: os.Getenv("WEB_DIR")}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.Database == "" {
		cfg.Database = "singlesfinder"
	}
	if cfg.DemoMode {
		return cfg, nil
	}
	uri := strings.TrimSpace(os.Getenv("MONGODB_URI"))
	if uri == "" {
		return cfg, fmt.Errorf("set MONGODB_URI in backend/.env (or set DEMO_MODE=true)")
	}
	if strings.Contains(uri, "<") || strings.Contains(uri, ">") {
		return cfg, fmt.Errorf("MONGODB_URI still contains placeholders")
	}
	parsed, err := url.Parse(uri)
	if err != nil || (parsed.Scheme != "mongodb+srv" && parsed.Scheme != "mongodb") || parsed.Host == "" {
		return cfg, fmt.Errorf("MONGODB_URI must be a valid mongodb+srv:// or mongodb:// connection string")
	}
	if parsed.Scheme == "mongodb+srv" {
		password, hasPassword := parsed.User.Password()
		if parsed.User.Username() == "" || !hasPassword || password == "" {
			return cfg, fmt.Errorf("MONGODB_URI needs a database username and password")
		}
	}
	cfg.MongoURI = uri
	return cfg, nil
}
