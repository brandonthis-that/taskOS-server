// Package config loads server configuration from the environment.
//
// Variables are read from the process environment, falling back to a `.env`
// file in the working directory when present.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB           DB
	HTTPAddr     string
	SessionHours int
}

type DB struct {
	User     string
	Password string
	Name     string
	Host     string
	Port     string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	c := Config{
		DB: DB{
			User:     get("DB_USER", ""),
			Password: get("DB_PASSWORD", ""),
			Name:     get("DB_NAME", ""),
			Host:     get("DB_HOST", "localhost"),
			Port:     get("DB_PORT", "5432"),
		},
		HTTPAddr:     get("HTTP_ADDR", ":8080"),
		SessionHours: getInt("SESSION_HOURS", 24*30),
	}

	if c.DB.User == "" || c.DB.Password == "" || c.DB.Name == "" {
		return Config{}, errors.New("DB_USER, DB_PASSWORD, and DB_NAME must be set")
	}
	return c, nil
}

// DSN builds a libpq-style connection string. url.UserPassword escapes
// special characters (e.g. '@') in the password.
func (d DB) DSN() string {
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     fmt.Sprintf("%s:%s", d.Host, d.Port),
		Path:     "/" + d.Name,
		RawQuery: url.Values{"sslmode": {"disable"}}.Encode(),
	}).String()
}

func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
