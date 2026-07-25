package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	requestTimeout = 10 * time.Second

	motivationPrompt = `
Generate one short motivational message.

Requirements:
- maximum two sentences;
- no greeting;
- no markdown;
- return only the motivational message;
- use the following language: %s;
- add light sarcasm and mockery, but do not use insults.
`
)

type Config struct {
	GeminiAPIKey               string `json:"gemini_api_key"`
	Model                      string `json:"model"`
	NotificationIntervalMinute int    `json:"notification_interval_minutes"`
	Language                   string `json:"language"`
}

type Client struct {
	client   *genai.Client
	model    string
	language string
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var config Config

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	config.normalize()

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config file %q: %w", path, err)
	}

	return &config, nil
}

func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client, err := genai.NewClient(
		context.Background(),
		&genai.ClientConfig{
			APIKey:  config.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return &Client{
		client:   client,
		model:    config.Model,
		language: config.Language,
	}, nil
}

func (c *Client) CreateRequest() (string, error) {
	if c == nil || c.client == nil {
		return "", errors.New("client is empty")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		requestTimeout,
	)
	defer cancel()

	prompt := fmt.Sprintf(
		motivationPrompt,
		c.language,
	)

	contents := []*genai.Content{
		genai.NewContentFromText(
			prompt,
			genai.RoleUser,
		),
	}

	response, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		contents,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("generate motivation: %w", err)
	}

	message := strings.TrimSpace(response.Text())
	if message == "" {
		return "", errors.New("an empty response")
	}

	return message, nil
}

func (c *Config) normalize() {
	c.GeminiAPIKey = strings.TrimSpace(c.GeminiAPIKey)
	c.Model = strings.TrimSpace(c.Model)
	c.Language = strings.TrimSpace(c.Language)

	if c.Language == "" {
		c.Language = "English"
	}
}

func (c *Config) validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	if c.GeminiAPIKey == "" {
		return errors.New("api_key empty")
	}

	if c.Model == "" {
		return errors.New("model is empty")
	}

	if c.NotificationIntervalMinute <= 0 {
		return errors.New(
			"notification_interval_minutes must be",
		)
	}

	return nil
}
