package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type bootstrapRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	userServiceURL := flag.String("url", envOr("USER_SERVICE_URL", "http://127.0.0.1:8082"),
		"base URL of the User Service")
	email := flag.String("email", "", "administrator email address")
	firstName := flag.String("first-name", "", "administrator first name")
	lastName := flag.String("last-name", "", "administrator last name")
	password := flag.String("password", os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"),
		"initial administrator password (prefer ADMIN_BOOTSTRAP_PASSWORD)")
	flag.Parse()

	token := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_TOKEN"))
	if token == "" {
		return fmt.Errorf("ADMIN_BOOTSTRAP_TOKEN must be configured")
	}
	if strings.TrimSpace(*email) == "" ||
		strings.TrimSpace(*firstName) == "" ||
		strings.TrimSpace(*lastName) == "" {
		return fmt.Errorf("-email, -first-name, and -last-name are required")
	}
	if *password == "" {
		return fmt.Errorf("-password or ADMIN_BOOTSTRAP_PASSWORD is required")
	}

	payload, err := json.Marshal(bootstrapRequest{
		Email:     *email,
		FirstName: *firstName,
		LastName:  *lastName,
		Password:  *password,
	})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	url := strings.TrimRight(*userServiceURL, "/") + "/internal/bootstrap"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call user service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("user service returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Println(strings.TrimSpace(string(body)))
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
