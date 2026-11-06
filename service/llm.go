package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-knowledge-assistant/config"
)

type LLM struct {
	Config config.Config
	Client *http.Client
}
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (l LLM) Complete(ctx context.Context, system, input string) (string, error) {
	if l.Config.LLMAPIKey == "" {
		return "", fmt.Errorf("LLM_API_KEY is not configured")
	}
	body, _ := json.Marshal(chatRequest{Model: l.Config.LLMModel, Messages: []chatMessage{{"system", system}, {"user", input}}, Temperature: 0})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.Config.LLMBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	cl := l.Client
	if cl == nil {
		cl = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("LLM request failed: %s", res.Status)
	}
	var parsed struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
	}
	if err = json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("invalid LLM response")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
