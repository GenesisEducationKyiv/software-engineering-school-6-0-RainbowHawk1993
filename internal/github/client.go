package github

import (
	"context"
	"encoding/json"
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
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) error {
	response, err := c.doRequest(ctx, fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusOK:
		return nil
	case isRateLimited(response):
		return apperr.ErrRateLimited
	case response.StatusCode == http.StatusNotFound:
		return apperr.ErrRepoNotFound
	default:
		return unexpectedStatus(response)
	}
}

func (c *Client) LatestReleaseTag(ctx context.Context, owner, repo string) (string, error) {
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
			return "", nil
		}

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

var _ interface {
	RepoExists(context.Context, string, string) error
	LatestReleaseTag(context.Context, string, string) (string, error)
} = (*Client)(nil)
