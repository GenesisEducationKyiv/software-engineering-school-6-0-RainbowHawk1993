package github

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheTTL = 10 * time.Minute

var errCacheMiss = errors.New("cache miss")

type Cache interface {
	GetRepoExists(context.Context, string, string) (bool, error)
	SetRepoExists(context.Context, string, string, bool) error
	GetLatestReleaseTag(context.Context, string, string) (string, bool, error)
	SetLatestReleaseTag(context.Context, string, string, string, bool) error
}

type RedisCache struct {
	client *redis.Client
}

type releaseCacheValue struct {
	Tag   string `json:"tag"`
	Found bool   `json:"found"`
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) GetRepoExists(ctx context.Context, owner, repo string) (bool, error) {
	value, err := c.client.Get(ctx, repoExistsKey(owner, repo)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, errCacheMiss
		}
		return false, err
	}

	return value == "1", nil
}

func (c *RedisCache) SetRepoExists(ctx context.Context, owner, repo string, exists bool) error {
	value := "0"
	if exists {
		value = "1"
	}

	return c.client.Set(ctx, repoExistsKey(owner, repo), value, cacheTTL).Err()
}

func (c *RedisCache) GetLatestReleaseTag(ctx context.Context, owner, repo string) (string, bool, error) {
	value, err := c.client.Get(ctx, latestReleaseKey(owner, repo)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, errCacheMiss
		}
		return "", false, err
	}

	var cached releaseCacheValue
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return "", false, err
	}

	return cached.Tag, cached.Found, nil
}

func (c *RedisCache) SetLatestReleaseTag(ctx context.Context, owner, repo, tag string, found bool) error {
	value, err := json.Marshal(releaseCacheValue{Tag: tag, Found: found})
	if err != nil {
		return err
	}

	return c.client.Set(ctx, latestReleaseKey(owner, repo), value, cacheTTL).Err()
}

func repoExistsKey(owner, repo string) string {
	return "github:repo_exists:" + owner + "/" + repo
}

func latestReleaseKey(owner, repo string) string {
	return "github:latest_release:" + owner + "/" + repo
}
