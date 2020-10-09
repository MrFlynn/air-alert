package internal

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"golang.org/x/crypto/acme/autocert"
)

// RedisCertificateCache is used to store program certificate data inside redis.
type RedisCertificateCache struct {
	client *redis.Client
}

// NewCache initializes a new RedisCertificateCache struct.
func NewCache() (*RedisCertificateCache, error) {
	var id int
	for ; id < 16; id++ {
		// Find ID that hasn't been taken.
		if viper.GetInt("database.redis.id") != id {
			break
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("database.redis.addr"),
		Password: viper.GetString("database.redis.password"),
		DB:       id,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &RedisCertificateCache{
		client: client,
	}, nil
}

// Get returns corresponding value for the key in the datastore.
func (r *RedisCertificateCache) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		// If nothing is found, return cache miss.
		return nil, autocert.ErrCacheMiss
	} else if err != nil {
		// If something else happens then we should return the actual error.
		return nil, err
	}

	return value, nil
}

// Put stores the corresponding data in the datastore.
func (r *RedisCertificateCache) Put(ctx context.Context, key string, data []byte) error {
	return r.client.Set(ctx, key, data, 0).Err()
}

// Delete removes the data stored at the given key.
func (r *RedisCertificateCache) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err == redis.Nil {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
