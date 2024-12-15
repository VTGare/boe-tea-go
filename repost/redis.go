package repost

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisDetector struct {
	client *redis.Client
}

func NewRedis(addr string) (Detector, error) {
	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		MaxRetries: 5,
	})

	status := client.Ping(context.Background())
	if status.Err() != nil {
		return nil, status.Err()
	}

	return &redisDetector{client}, nil
}

func (rd redisDetector) exists(ctx context.Context, key string) error {
	exists, err := rd.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		return ErrNotFound
	}

	return nil
}

func (rd redisDetector) Find(ctx context.Context, channelID, artworkID string) (*Repost, error) {
	var (
		rep Repost
		key = fmt.Sprintf("channel:%v:artwork:%v", channelID, artworkID)
	)

	if err := rd.exists(ctx, key); err != nil {
		return nil, err
	}

	var (
		repostResult *redis.StringStringMapCmd
		ttlResult    *redis.DurationCmd
	)

	_, err := rd.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		repostResult = pipe.HGetAll(ctx, key)
		if err := repostResult.Err(); err != nil {
			return err
		}

		ttlResult = pipe.TTL(ctx, key)
		return ttlResult.Err()
	})
	if err != nil {
		return nil, err
	}

	if err := repostResult.Scan(&rep); err != nil {
		return nil, err
	}

	rep.ExpiresAt = time.Now().Add(ttlResult.Val())
	return &rep, nil
}

func (rd redisDetector) Create(ctx context.Context, repost *Repost, duration time.Duration) error {
	key := fmt.Sprintf("channel:%v:artwork:%v", repost.ChannelID, repost.ID)
	_, err := rd.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		if _, err := pipe.HSet(ctx, key, map[string]any{
			"id":         repost.ID,
			"url":        repost.URL,
			"guild_id":   repost.GuildID,
			"channel_id": repost.ChannelID,
			"message_id": repost.MessageID,
		}).Result(); err != nil {
			return err
		}

		if _, err := pipe.ExpireAt(ctx, key, time.Now().Add(duration)).Result(); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (rd redisDetector) Delete(ctx context.Context, channelID, artworkID string) error {
	key := fmt.Sprintf("channel:%v:artwork:%v", channelID, artworkID)

	if err := rd.exists(ctx, key); err != nil {
		return err
	}

	if _, err := rd.client.Del(ctx, key).Result(); err != nil {
		return err
	}

	return nil
}

func (rd redisDetector) Close() error {
	return rd.client.Close()
}
