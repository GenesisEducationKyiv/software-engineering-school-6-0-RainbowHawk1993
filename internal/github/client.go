package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"releasesapi/internal/apperr"
)

const baseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	token      string
	cache      Cache
	httpClient *http.Client
}

func NewClient(token string, cache Cache) *Client {
	return &Client{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		cache:   cache,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) error {
	if c.cache != nil {
		exists, err := c.cache.GetRepoExists(ctx, owner, repo)
		switch {
		case err == nil && exists:
			return nil
		case err == nil && !exists:
			return apperr.ErrRepoNotFound
		case errors.Is(err, errCacheMiss):
		case err != nil:
		}
	}

	response, err := c.doRequest(ctx, fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusOK:
		c.cacheRepoExists(ctx, owner, repo, true)
		return nil
	case isRateLimited(response):
		return apperr.ErrRateLimited
	case response.StatusCode == http.StatusNotFound:
		c.cacheRepoExists(ctx, owner, repo, false)
		return apperr.ErrRepoNotFound
	default:
		return unexpectedStatus(response)
	}
}

func (c *Client) LatestReleaseTag(ctx context.Context, owner, repo string) (string, error) {
	if c.cache != nil {
		tag, found, err := c.cache.GetLatestReleaseTag(ctx, owner, repo)
		switch {
		case err == nil && found:
			return tag, nil
		case err == nil && !found:
			return "", nil
		case errors.Is(err, errCacheMiss):
		case err != nil:
		}
	}

	response, err := c.doRequest(ctx, fmt.Sprintf("%s/repos/%s/%s/releases?per_page=1", c.baseURL, owner, repo))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusOK:
		var releases []struct {
			TagName string `json:"tag_name"`
		}

		if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
			return "", err
		}
		if len(releases) == 0 {
			c.cacheLatestRelease(ctx, owner, repo, "", false)
			return "", nil
		}

		c.cacheLatestRelease(ctx, owner, repo, releases[0].TagName, true)
		return releases[0].TagName, nil
	case isRateLimited(response):
		return "", apperr.ErrRateLimited
	case response.StatusCode == http.StatusNotFound:
		return "", apperr.ErrRepoNotFound
	default:
		return "", unexpectedStatus(response)
	}
}

func (c *Client) doRequest(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "releases-api")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(request)
}

func isRateLimited(response *http.Response) bool {
	if response.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0"
}

func unexpectedStatus(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, 512))
	if err != nil {
		return fmt.Errorf("github unexpected status %d", response.StatusCode)
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("github unexpected status %d", response.StatusCode)
	}

	return fmt.Errorf("github unexpected status %d: %s", response.StatusCode, message)
}

func (c *Client) cacheRepoExists(ctx context.Context, owner, repo string, exists bool) {
	if c.cache == nil {
		return
	}
	_ = c.cache.SetRepoExists(ctx, owner, repo, exists)
}

func (c *Client) cacheLatestRelease(ctx context.Context, owner, repo, tag string, found bool) {
	if c.cache == nil {
		return
	}
	_ = c.cache.SetLatestReleaseTag(ctx, owner, repo, tag, found)
}

var _ interface {
	RepoExists(context.Context, string, string) error
	LatestReleaseTag(context.Context, string, string) (string, error)
} = (*Client)(nil)
