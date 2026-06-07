package github

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"releasesapi/internal/apperr"
)

type fakeCache struct {
	repoValue        bool
	repoErr          error
	repoSetExists    *bool
	releaseTag       string
	releaseFound     bool
	releaseErr       error
	releaseSetTag    string
	releaseSetFound  bool
	releaseSetCalled bool
}

func (f *fakeCache) GetRepoExists(context.Context, string, string) (bool, error) {
	return f.repoValue, f.repoErr
}

func (f *fakeCache) SetRepoExists(_ context.Context, _, _ string, exists bool) error {
	value := exists
	f.repoSetExists = &value
	return nil
}

func (f *fakeCache) GetLatestReleaseTag(context.Context, string, string) (string, bool, error) {
	return f.releaseTag, f.releaseFound, f.releaseErr
}

func (f *fakeCache) SetLatestReleaseTag(_ context.Context, _, _, tag string, found bool) error {
	f.releaseSetTag = tag
	f.releaseSetFound = found
	f.releaseSetCalled = true
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRepoExistsUsesCacheHit(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := &fakeCache{repoValue: true}
	client := NewClient("", cache, nil, logger)
	called := false
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, errors.New("unexpected network call")
		}),
	}

	if err := client.RepoExists(context.Background(), "owner", "repo"); err != nil {
		t.Fatalf("RepoExists returned error: %v", err)
	}
	if called {
		t.Fatal("expected cache hit to avoid network call")
	}
}

func TestRepoExistsCachesNotFound(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := &fakeCache{repoErr: errCacheMiss}
	client := NewClient("", cache, nil, logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	err := client.RepoExists(context.Background(), "owner", "repo")
	if !errors.Is(err, apperr.ErrRepoNotFound) {
		t.Fatalf("expected ErrRepoNotFound, got %v", err)
	}
	if cache.repoSetExists == nil || *cache.repoSetExists {
		t.Fatal("expected not-found repo result to be cached")
	}
}

func TestLatestReleaseTagUsesCacheHit(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := &fakeCache{
		releaseTag:   "v1.2.3",
		releaseFound: true,
	}
	client := NewClient("", cache, nil, logger)
	called := false
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, errors.New("unexpected network call")
		}),
	}

	tag, err := client.LatestReleaseTag(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("LatestReleaseTag returned error: %v", err)
	}
	if tag != "v1.2.3" {
		t.Fatalf("expected cached tag, got %q", tag)
	}
	if called {
		t.Fatal("expected cache hit to avoid network call")
	}
}

func TestLatestReleaseTagCachesNoRelease(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := &fakeCache{releaseErr: errCacheMiss}
	client := NewClient("", cache, nil, logger)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("[]")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	tag, err := client.LatestReleaseTag(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("LatestReleaseTag returned error: %v", err)
	}
	if tag != "" {
		t.Fatalf("expected empty tag, got %q", tag)
	}
	if !cache.releaseSetCalled || cache.releaseSetFound {
		t.Fatal("expected empty latest release result to be cached")
	}
}
