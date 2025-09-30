package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/airenas/go-app/pkg/goapp"
	"github.com/airenas/rt-transcriber-wrapper/internal/domain"
	"github.com/airenas/rt-transcriber-wrapper/internal/secure"
	"github.com/redis/go-redis/v9"
)

// RedisDataManager stores audio, user configs, and texts in Redis.
type RedisDataManager struct {
	client  *redis.Client
	ttl     time.Duration
	crypter *secure.Crypter
}

// NewRedisDataManager creates a new RedisDataManager with connection pooling.
func NewRedisDataManager(connStr string, encryptionKey string) (*RedisDataManager, error) {
	opt, err := redis.ParseURL(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	goapp.Log.Info().Str("redis", opt.Addr).Int("db", opt.DB).Send()
	rdb := redis.NewClient(opt)

	crypter, err := secure.NewCrypter(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create crypter: %w", err)
	}

	return &RedisDataManager{
		client:  rdb,
		ttl:     time.Hour * 6,
		crypter: crypter,
	}, nil
}

func (r *RedisDataManager) keyAudio(id string) string {
	return fmt.Sprintf("audio:%s", id)
}

func (r *RedisDataManager) keyConfig(id string) string {
	return fmt.Sprintf("user:%s", id)
}

func (r *RedisDataManager) keyTexts(id string) string {
	return fmt.Sprintf("texts:%s", id)
}

// SaveAudio stores WAV bytes in Redis
func (r *RedisDataManager) SaveAudio(ctx context.Context, id string, chunks [][]byte) error {
	goapp.Log.Trace().Str("id", id).Msg("Save audio")

	data, err := to_wav(chunks)
	if err != nil {
		return fmt.Errorf("convert to wav: %w", err)
	}
	encrypted, err := r.crypter.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	key := r.keyAudio(id)
	return r.client.Set(ctx, key, encrypted, r.ttl).Err()
}

// GetAudio retrieves WAV bytes from Redis
func (r *RedisDataManager) GetAudio(ctx context.Context, id string) ([]byte, error) {
	goapp.Log.Trace().Str("id", id).Msg("Get audio")
	key := r.keyAudio(id)
	b, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}
	decrypted, err := r.crypter.Decrypt(b)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return decrypted, nil
}

// SaveConfig stores user config in Redis as JSON
func (r *RedisDataManager) SaveConfig(ctx context.Context, user *domain.User) error {
	key := r.keyConfig(user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	encrypted, err := r.crypter.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	return r.client.Set(ctx, key, encrypted, 0).Err()
}

// GetConfig retrieves user config from Redis
func (r *RedisDataManager) GetConfig(ctx context.Context, userID string) (*domain.User, error) {
	key := r.keyConfig(userID)
	bs, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return &domain.User{ID: userID}, nil
		}
		return nil, fmt.Errorf("get config: %w", err)
	}
	decrypted, err := r.crypter.Decrypt(bs)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var u domain.User
	if err := json.Unmarshal(decrypted, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SaveTexts stores Texts in Redis as JSON
func (r *RedisDataManager) SaveTexts(ctx context.Context, userID string, input *domain.Texts) error {
	key := r.keyTexts(userID)
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	encrypted, err := r.crypter.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	return r.client.Set(ctx, key, encrypted, r.ttl).Err()
}

// GetTexts retrieves Texts from Redis
func (r *RedisDataManager) GetTexts(ctx context.Context, userID string) (*domain.Texts, error) {
	key := r.keyTexts(userID)
	bs, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return &domain.Texts{}, nil
		}
		return nil, fmt.Errorf("get texts: %w", err)
	}
	decrypted, err := r.crypter.Decrypt(bs)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var t domain.Texts
	if err := json.Unmarshal(decrypted, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *RedisDataManager) Close() error {
	return r.client.Close()
}
