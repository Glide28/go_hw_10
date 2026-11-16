package redis

import (
	"context"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

type Cache struct{ client *redisclient.Client }

func New(ctx context.Context, address, password string, database int) (*Cache, error) {
	client := redisclient.NewClient(&redisclient.Options{Addr: address, Password: password, DB: database})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Cache{client: client}, nil
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key).Bytes()
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *Cache) Close() error { return c.client.Close() }
