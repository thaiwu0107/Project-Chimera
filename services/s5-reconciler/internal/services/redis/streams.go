package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// StreamMessage represents a generic message payload for Redis Streams
type StreamMessage map[string]interface{}

// PublishStream publishes a message to a Redis Stream
func (r *RedisClient) PublishStream(ctx context.Context, streamName string, message StreamMessage) (string, error) {
	args := make([]interface{}, 0, len(message)*2)
	for k, v := range message {
		args = append(args, k, v)
	}
	cmd := r.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		MaxLen: 1000000, // Example max length, should be configurable
		Values: args,
	})
	return cmd.Result()
}

// ConsumeStream reads messages from a Redis Stream using a consumer group
func (r *RedisClient) ConsumeStream(ctx context.Context, streamName, groupName, consumerName string, count int64, block time.Duration) ([]redis.XStream, error) {
	streams, err := r.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"}, // ">" means new messages
		Count:    count,
		Block:    block,
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to consume stream %s: %w", streamName, err)
	}
	return streams, nil
}

// CreateConsumerGroup creates a consumer group for a given stream
func (r *RedisClient) CreateConsumerGroup(ctx context.Context, streamName, groupName string) error {
	_, err := r.Client.XGroupCreateMkStream(ctx, streamName, groupName, "0").Result()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("failed to create consumer group %s for stream %s: %w", groupName, streamName, err)
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		log.Printf("Consumer group %s already exists for stream %s", groupName, streamName)
	}
	return nil
}

// AcknowledgeStreamMessage acknowledges a message in a stream
func (r *RedisClient) AcknowledgeStreamMessage(ctx context.Context, streamName, groupName string, ids ...string) error {
	_, err := r.Client.XAck(ctx, streamName, groupName, ids...).Result()
	if err != nil {
		return fmt.Errorf("failed to acknowledge messages in stream %s for group %s: %w", streamName, groupName, err)
	}
	return nil
}
