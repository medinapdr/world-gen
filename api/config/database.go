package config

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// DatabaseConfig manages database connections
type DatabaseConfig struct {
	DB          *pgxpool.Pool
	RedisClient *redis.Client
}

// NewDatabaseConfig creates a new database configuration instance
func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{}
}

// ConnectPostgres connects to PostgreSQL database
func (c *DatabaseConfig) ConnectPostgres() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("DATABASE_URL is not configured. Skipping PostgreSQL connection.")
		return nil
	}

	var err error
	c.DB, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return err
	}

	err = c.DB.Ping(context.Background())
	if err != nil {
		return err
	}

	log.Println("Successfully connected to PostgreSQL.")
	return nil
}

// ConnectRedis connects to Redis
func (c *DatabaseConfig) ConnectRedis() error {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	c.RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err := c.RedisClient.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	log.Println("Successfully connected to Redis.")
	return nil
}

// Close closes all connections
func (c *DatabaseConfig) Close() {
	if c.DB != nil {
		c.DB.Close()
	}

	if c.RedisClient != nil {
		c.RedisClient.Close()
	}
}
