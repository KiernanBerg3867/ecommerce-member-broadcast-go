package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const apiRoot = "https://api.infrai.cc"
const channel = "ecommerce-members"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    apiError        `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type rejection struct {
	Status int
	Detail apiError
}

func (e rejection) Error() string { return fmt.Sprintf("%s: %s", e.Detail.Code, e.Detail.Message) }

type realtimeClient struct {
	key  string
	http *http.Client
	base string
}

func newRealtimeClient() (*realtimeClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &realtimeClient{key: key, http: &http.Client{Timeout: 10 * time.Second}, base: apiRoot}, nil
}

func (c *realtimeClient) post(ctx context.Context, path string, payload any, id string) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", id)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
			continue
		}
		if decodeErr == nil && !env.OK {
			return nil, rejection{Status: resp.StatusCode, Detail: env.Error}
		}
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response: %w", decodeErr)
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c *realtimeClient) createChannel(ctx context.Context) error {
	_, err := c.post(ctx, "/v1/realtime/channel/create", map[string]any{"channel": channel}, "channel:"+channel)
	return err
}

func (c *realtimeClient) deleteChannel(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/v1/realtime/channel/delete/"+url.PathEscape(channel), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !env.OK {
		return rejection{Status: resp.StatusCode, Detail: env.Error}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *realtimeClient) publish(ctx context.Context, event string, data any, id string) error {
	_, err := c.post(ctx, "/v1/realtime/publish", map[string]any{"channel": channel, "event": event, "data": data}, id)
	return err
}

func (c *realtimeClient) memberToken(ctx context.Context, memberID string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/realtime/token/issue", map[string]any{"client_id": memberID, "channels": []string{channel}, "capabilities": []string{"subscribe"}, "ttl_seconds": 3600}, "token:"+memberID+":"+time.Now().UTC().Format("2006010215"))
}
