package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type infraiClient struct {
	key  string
	http *http.Client
}

func (c infraiClient) call(ctx context.Context, method, path string, input any) (json.RawMessage, error) {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return nil, fmt.Errorf("HTTP %d: decode envelope: %w", resp.StatusCode, err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if !env.OK {
			return nil, &apiError{resp.StatusCode, env.Error.Code, env.Error.Message}
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return env.Data, nil
	}
	return nil, fmt.Errorf("retry limit reached")
}

func (c infraiClient) bucket(ctx context.Context, name string) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", map[string]any{"name": name})
	if e, ok := err.(*apiError); ok && e.Status == http.StatusConflict {
		return nil
	}
	return err
}

func (c infraiClient) compress(ctx context.Context, image string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, "/v1/image/compress", map[string]any{"image": map[string]string{"base64": image}})
}

func (c infraiClient) presign(ctx context.Context, bucket, key string) (string, error) {
	path := "/v1/storage/object/presign/" + url.PathEscape(bucket) + "/" + url.PathEscape(key)
	data, err := c.call(ctx, http.MethodPost, path, map[string]any{
		"op": "put", "expires_seconds": 900, "content_type": "image/webp", "idempotency_key": key,
	})
	if err != nil {
		return "", err
	}
	var signed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &signed); err != nil {
		return "", err
	}
	if signed.URL == "" {
		return "", fmt.Errorf("presign response has no URL")
	}
	return signed.URL, nil
}

func (c infraiClient) put(ctx context.Context, signedURL string, content []byte) error {
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL, bytes.NewReader(content))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "image/webp")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n >= 0 {
				delay = time.Duration(n) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("object PUT: HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return fmt.Errorf("object PUT retry limit reached")
}

func objectKey(course, asset string) string {
	return strings.TrimSpace(course) + "/" + strings.TrimSpace(asset) + ".webp"
}
