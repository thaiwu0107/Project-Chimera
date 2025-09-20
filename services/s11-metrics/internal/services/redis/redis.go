package redis

import (
	"context"
	"fmt"
	"log"
	"s11-metrics/internal/config"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	redisOnce   sync.Once
	RInstance   *RedisClient
	redisCancel context.CancelFunc
)

type RedisClient struct {
	Client *redis.ClusterClient
	Ctx    context.Context
}

func GetInstance() *RedisClient {
	return RInstance
}

func Init() error {
	var err error
	redisOnce.Do(func() {
		cfg := config.AppConfig.Redis
		if cfg.Addr == "" {
			err = fmt.Errorf("redis address not configured")
			return
		}

		redisIPs := strings.Split(cfg.Addr, ",")
		redisC := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    redisIPs,
			Password: cfg.Password,
			PoolSize: cfg.PoolSize,
		})

		ctx, cancel := context.WithCancel(context.Background())
		redisCancel = cancel

		if _, pingErr := redisC.Ping(ctx).Result(); pingErr != nil {
			err = fmt.Errorf("redis ping failed: %w", pingErr)
			return
		}

		go func(c redis.Cmdable, ctx context.Context) {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					log.Println("Redis ping goroutine stopped")
					return
				case <-ticker.C:
					if _, pingErr := c.Ping(context.TODO()).Result(); pingErr != nil {
						log.Printf("Redis ping failed: %v", pingErr)
					}
				}
			}
		}(redisC, ctx)

		RInstance = &RedisClient{
			Client: redisC,
			Ctx:    ctx,
		}
		log.Printf("Redis connect successed, cluster: %v", redisIPs)
	})
	return err
}

func Stop() {
	if redisCancel != nil {
		redisCancel()
		log.Println("Redis goroutines stopped")
	}
	if RInstance != nil && RInstance.Client != nil {
		RInstance.Client.Close()
		log.Println("Redis client closed")
	}
}
