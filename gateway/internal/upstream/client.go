package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeoutSeconds int) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

func (c *Client) ListVoices(ctx context.Context) ([]Voice, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/api/tts/list")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build voices request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list voices request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list voices returned status %d", resp.StatusCode)
	}

	var voices []Voice
	if err := json.NewDecoder(resp.Body).Decode(&voices); err != nil {
		return nil, fmt.Errorf("decode voices: %w", err)
	}

	return voices, nil
}

func (c *Client) Synthesize(ctx context.Context, params SynthesizeParams) (*http.Response, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/api/tts")

	values := endpoint.Query()
	values.Set("text", params.Text)
	values.Set("voiceName", params.Voice)
	values.Set("thread", strconv.Itoa(params.Thread))
	values.Set("shardLength", strconv.Itoa(params.ShardLength))
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build synthesize request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("synthesize request: %w", err)
	}

	return resp, nil
}
