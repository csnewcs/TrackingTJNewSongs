package main

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

func LoadConfig() Config {
	return Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "tj"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "tj"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func (c Config) ConnString() string {
	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBName, c.DBSSLMode)
	if c.DBPassword != "" {
		connStr += fmt.Sprintf(" password=%s", c.DBPassword)
	}
	return connStr
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
