package database // Hoặc một package khác nếu bạn muốn tách biệt hoàn toàn

import (
	"context"
	"fmt"
	"time"
    "crypto/tls" // Import package crypto/tls
	"github.com/go-redis/redis/v8" // Import thư viện Redis
	"services_app/internal/config" // Cần đảm bảo file config của bạn có struct RedisConfig (hoặc DatabaseConfig chứa thông tin Redis)
)

// --- Giả định về cấu trúc config.RedisConfig ---
// Bạn cần cập nhật struct này trong package config của mình
/*
type RedisConfig struct {
	RedisHost     string `yaml:"redis_host"`
	RedisPort     string `yaml:"redis_port"`
	RedisPassword string `yaml:"redis_password"` // Mật khẩu Redis (nếu có)
	RedisDB       int    `yaml:"redis_db"`       // Database index (thường là 0)
	// Có thể thêm các cấu hình Redis khác ở đây như PoolSize, v.v.
}

// Hoặc nếu bạn dùng chung DatabaseConfig:
/*
type DatabaseConfig struct {
	// ... cấu hình Postgres ...

	// Cấu hình Redis - THÊM CÁC TRƯỜNG NÀY
	RedisHost     string `yaml:"redis_host"`
	RedisPort     string `yaml:"redis_port"`
	RedisPassword string `yaml:"redis_password"` // Mật khẩu Redis (nếu có)
	RedisDB       int    `yaml:"redis_db"`       // Database index (thường là 0)
	// Có thể thêm các cấu hình Redis khác ở đây như PoolSize, v.v.
}
*/
// -------------------------------------------------


// RedisClient struct chứa client kết nối đến Redis
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient khởi tạo và kết nối đến Redis
// Hàm này có thể nhận struct config.RedisConfig hoặc config.DatabaseConfig tùy cách bạn tổ chức config
func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) { // Sử dụng config.DatabaseConfig như ví dụ trước
// func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) { // Hoặc sử dụng struct RedisConfig riêng
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Address,cfg.Port), // Sử dụng thông tin từ config
		Password: cfg.Password, // no password set if empty
		DB:       cfg.DB,       // use specified DB index
		// Có thể thêm các tùy chọn khác ở đây như PoolSize, ReadTimeout, v.v.
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // Nên chỉ định phiên bản TLS tối thiểu để tăng cường bảo mật
			// InsecureSkipVerify: true, // KHÔNG NÊN sử dụng trong môi trường production
		},
	})

	// Test the Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("error connecting to redis: %v", err)
	}

	return &RedisClient{client: redisClient}, nil
}

// Close đóng kết nối Redis
func (rc *RedisClient) Close() error {
	if rc.client != nil {
		// redisClient.Close() trả về error, nên có thể cần xử lý nếu muốn log lỗi.
		return rc.client.Close()
	}
	return nil
}

// GetClient trả về client Redis gốc để thực hiện các thao tác
func (rc *RedisClient) GetClient() *redis.Client {
	return rc.client
}

// Bạn có thể thêm các phương thức wrapper cho các thao tác Redis phổ biến ở đây
// Ví dụ:

func (rc *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return rc.client.Set(ctx, key, value, expiration)
}

func (rc *RedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	return rc.client.Get(ctx, key)
}

func (rc *RedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return rc.client.Del(ctx, keys...)
}

func (rc *RedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return rc.client.Exists(ctx, keys...)
}
