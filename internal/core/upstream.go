package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Event represents a long-polling event from Laravel
type Event struct {
	ID        int64                  `json:"id"`
	Event     map[string]interface{} `json:"event"`
	CreatedAt int64                  `json:"created_at"`
}

// LaravelResponse represents the response from Laravel's /getEvents endpoint
type LaravelResponse struct {
	Events []Event `json:"events"`
	Count  int     `json:"count"`
}

// LaravelUpstreamPool manages concurrent requests to Laravel
type LaravelUpstreamPool struct {
	laravelAddr string
	secret      string
	maxLimit    int
	logger      *slog.Logger
	semaphore   chan struct{}
	httpClient  *http.Client
}

// NewLaravelUpstreamPool creates a new Laravel upstream pool
func NewLaravelUpstreamPool(
	laravelAddr string,
	secret string,
	maxLimit int,
	workers int,
	requestTimeout time.Duration,
	maxIdleConns int,
	maxConnsPerHost int,
	idleConnTimeout time.Duration,
	logger *slog.Logger,
) *LaravelUpstreamPool {
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxConnsPerHost,
		MaxConnsPerHost:     maxConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	}

	return &LaravelUpstreamPool{
		laravelAddr: laravelAddr,
		secret:      secret,
		maxLimit:    maxLimit,
		logger:      logger,
		semaphore:   make(chan struct{}, workers),
		httpClient: &http.Client{
			Timeout:   requestTimeout,
			Transport: transport,
		},
	}
}

// GetEvents fetches events from Laravel for a specific channel
func (p *LaravelUpstreamPool) GetEvents(ctx context.Context, channelID string, offset int64, limit int) ([]Event, error) {
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if limit > p.maxLimit {
		limit = p.maxLimit
	}

	reqURL := fmt.Sprintf("%s/api/long-polling/getEvents?channel_id=%s&secret=%s&offset=%d&limit=%d",
		p.laravelAddr,
		url.QueryEscape(channelID),
		url.QueryEscape(p.secret),
		offset,
		limit,
	)

	p.logger.Debug("fetching events from Laravel",
		"url", reqURL,
		"channel_id", channelID,
		"offset", offset,
		"limit", limit,
	)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Laravel returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var laravelResp LaravelResponse
	if err := json.NewDecoder(resp.Body).Decode(&laravelResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	p.logger.Debug("received events from Laravel",
		"channel_id", channelID,
		"count", laravelResp.Count,
	)

	return laravelResp.Events, nil
}
