package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	dir            string
	disableBrotli  bool
	disableGzip    bool
	disableLogging bool
	enableNgrok    bool
	passwordStdin  bool
	port           string
	user           string
}

func (cfg *Config) validate() error {
	if err := cfg.validateDir(); err != nil {
		return fmt.Errorf("directory validation failed: %w", err)
	}
	if err := cfg.validatePort(); err != nil {
		return fmt.Errorf("port validation failed: %w", err)
	}
	if err := cfg.validateUser(); err != nil {
		return fmt.Errorf("user validation failed: %w", err)
	}
	return nil
}

func (cfg *Config) validateDir() error {
	info, err := os.Stat(cfg.dir)
	if err != nil {
		return fmt.Errorf("failed to access the directory %s: %w", cfg.dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", cfg.dir)
	}

	testFile, err := os.CreateTemp(cfg.dir, "")
	if err != nil {
		return fmt.Errorf("directory %s is not writable: %w", cfg.dir, err)
	}
	defer func(name string) {
		if err := os.Remove(name); err != nil {
			log.Printf("Failed to remove test file during directory validation: %s", err)
		}
	}(testFile.Name())

	return nil
}

func (cfg *Config) validateUser() error {
	if cfg.user == "" {
		return fmt.Errorf("user cannot be empty")
	}

	if strings.Contains(cfg.user, ":") {
		return fmt.Errorf("user cannot contain a colon")
	}

	if strings.TrimSpace(cfg.user) != cfg.user {
		return fmt.Errorf("user cannot have leading or trailing whitespace")
	}

	return nil
}

func (cfg *Config) validatePort() error {
	if cfg.port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	portNum, err := strconv.Atoi(cfg.port)
	if err != nil {
		return fmt.Errorf("port must be a number: %w", err)
	}

	if portNum < 0 || portNum > 65535 {
		return fmt.Errorf("port must be in the range 0-65535")
	}

	if cfg.port != "0" {
		listener, err := net.Listen("tcp4", ":"+cfg.port)
		if err != nil {
			return fmt.Errorf("port %s cannot be used: %w", cfg.port, err)
		}

		if err := listener.Close(); err != nil {
			return fmt.Errorf("port %s failed to close during validation: %w", cfg.port, err)
		}
	}

	return nil
}
