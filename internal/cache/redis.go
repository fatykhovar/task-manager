package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatykhovar/task-manager/internal/config"
	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(cfg config.RedisConfig) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Redis{client: client}, nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}

type TaskCache struct {
	redis *Redis
	ttl   time.Duration
}

func NewTaskCache(r *Redis, ttl time.Duration) *TaskCache {
	return &TaskCache{redis: r, ttl: ttl}
}

func (c *TaskCache) key(teamID int64, status string, page int) string {
	return fmt.Sprintf("tasks:team:%d:status:%s:page:%d", teamID, status, page)
}

func (c *TaskCache) GetTeamTasks(ctx context.Context, teamID int64, status string, page int) ([]*model.Task, bool) {
	val, err := c.redis.client.Get(ctx, c.key(teamID, status, page)).Result()
	if err != nil {
		return nil, false
	}
	var tasks []*model.Task
	if err := json.Unmarshal([]byte(val), &tasks); err != nil {
		return nil, false
	}
	return tasks, true
}

func (c *TaskCache) SetTeamTasks(ctx context.Context, teamID int64, status string, page int, tasks []*model.Task) {
	data, err := json.Marshal(tasks)
	if err != nil {
		return
	}
	c.redis.client.Set(ctx, c.key(teamID, status, page), data, c.ttl)
}

func (c *TaskCache) InvalidateTeam(ctx context.Context, teamID int64) {
	pattern := fmt.Sprintf("tasks:team:%d:*", teamID)
	keys, err := c.redis.client.Keys(ctx, pattern).Result()
	if err != nil || len(keys) == 0 {
		return
	}
	c.redis.client.Del(ctx, keys...)
}
