package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL               string
	Port                      string
	LogLevel                  string
	EnableEventLog            bool
	BalanceStalenessThreshold time.Duration
	RateLimitRPM              int
	JWTSecret                 string
	JWTExpiry                 time.Duration

	// DB connection pool tuning
	DBMaxConns              int32
	DBMinConns              int32
	DBMaxConnLifetime       time.Duration
	DBMaxConnIdleTime       time.Duration
	DBHealthCheckPeriod     time.Duration
	DBMaxConnLifetimeJitter time.Duration
	DBPingTimeout           time.Duration
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required but not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	enableEventLog := true
	if v := os.Getenv("ENABLE_EVENT_LOG"); v != "" {
		var err error
		enableEventLog, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("ENABLE_EVENT_LOG must be a boolean (true/false): %w", err)
		}
	}

	stalenessThreshold := 5 * time.Minute
	if v := os.Getenv("BALANCE_STALENESS_THRESHOLD"); v != "" {
		var err error
		stalenessThreshold, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("BALANCE_STALENESS_THRESHOLD must be a valid duration (e.g. 5m, 30s): %w", err)
		}
	}

	rateLimitRPM := 60
	if v := os.Getenv("RATE_LIMIT_RPM"); v != "" {
		var err error
		rateLimitRPM, err = strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("RATE_LIMIT_RPM must be an integer: %w", err)
		}
		if rateLimitRPM <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_RPM must be a positive integer, got %d", rateLimitRPM)
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required but not set")
	}

	jwtExpiry := 24 * time.Hour
	if v := os.Getenv("JWT_EXPIRY"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err == nil {
			jwtExpiry = parsed
		}
		// if unparseable, silently default to 24h per spec
	}

	dbMaxConns := int64(20)
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		var err error
		dbMaxConns, err = strconv.ParseInt(v, 10, 32)
		if err != nil || dbMaxConns <= 0 {
			return nil, fmt.Errorf("DB_MAX_CONNS must be a positive integer: %w", err)
		}
	}

	dbMinConns := int64(2)
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		var err error
		dbMinConns, err = strconv.ParseInt(v, 10, 32)
		if err != nil || dbMinConns < 0 {
			return nil, fmt.Errorf("DB_MIN_CONNS must be a non-negative integer: %w", err)
		}
	}

	dbMaxConnLifetime := 30 * time.Minute
	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		var err error
		dbMaxConnLifetime, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DB_MAX_CONN_LIFETIME must be a valid duration (e.g. 30m): %w", err)
		}
	}

	dbMaxConnIdleTime := 5 * time.Minute
	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		var err error
		dbMaxConnIdleTime, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DB_MAX_CONN_IDLE_TIME must be a valid duration (e.g. 5m): %w", err)
		}
	}

	dbHealthCheckPeriod := 1 * time.Minute
	if v := os.Getenv("DB_HEALTH_CHECK_PERIOD"); v != "" {
		var err error
		dbHealthCheckPeriod, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DB_HEALTH_CHECK_PERIOD must be a valid duration (e.g. 1m): %w", err)
		}
	}

	dbMaxConnLifetimeJitter := 5 * time.Minute
	if v := os.Getenv("DB_MAX_CONN_LIFETIME_JITTER"); v != "" {
		var err error
		dbMaxConnLifetimeJitter, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DB_MAX_CONN_LIFETIME_JITTER must be a valid duration (e.g. 5m): %w", err)
		}
	}

	dbPingTimeout := 5 * time.Second
	if v := os.Getenv("DB_PING_TIMEOUT"); v != "" {
		var err error
		dbPingTimeout, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DB_PING_TIMEOUT must be a valid duration (e.g. 5s): %w", err)
		}
	}

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		LogLevel:                  logLevel,
		EnableEventLog:            enableEventLog,
		BalanceStalenessThreshold: stalenessThreshold,
		RateLimitRPM:              rateLimitRPM,
		JWTSecret:                 jwtSecret,
		JWTExpiry:                 jwtExpiry,
		DBMaxConns:                int32(dbMaxConns),
		DBMinConns:                int32(dbMinConns),
		DBMaxConnLifetime:         dbMaxConnLifetime,
		DBMaxConnIdleTime:         dbMaxConnIdleTime,
		DBHealthCheckPeriod:       dbHealthCheckPeriod,
		DBMaxConnLifetimeJitter:   dbMaxConnLifetimeJitter,
		DBPingTimeout:             dbPingTimeout,
	}, nil
}
