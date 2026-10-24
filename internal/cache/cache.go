// Package cache implementa contadores, llaves de enfriamiento y caché corta
// sobre Redis, con respaldo en memoria local cuando Redis no responde. Es
// equivalente al CacheStore de la versión en Java (prefijo "smartpot:").
package cache

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix    = "smartpot:"
	redisBackoff = 30 * time.Second
	opTimeout    = 2 * time.Second
)

// NewRedisClient crea el cliente de Redis con los tiempos de espera dados.
func NewRedisClient(addr, username, password string, db int, timeout time.Duration) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Username:     username,
		Password:     password,
		DB:           db,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	})
}

// Store es el almacén con respaldo local. Si rdb es nil, siempre usa memoria.
type Store struct {
	rdb     *redis.Client
	now     func() time.Time
	mu      sync.Mutex
	local   map[string]localEntry
	retryAt time.Time
}

type localEntry struct {
	value     string
	expiresAt time.Time
}

func (e localEntry) isExpired(now time.Time) bool { return !now.Before(e.expiresAt) }

// NewStore crea el almacén. rdb puede ser nil (solo memoria local).
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb, now: time.Now, local: make(map[string]localEntry)}
}

// Increment incrementa el contador y fija su TTL la primera vez.
func (s *Store) Increment(key string, ttl time.Duration) int64 {
	full := keyPrefix + key
	if s.redisReady() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		val, err := s.rdb.Incr(ctx, full).Result()
		if err == nil {
			if val == 1 {
				s.rdb.Expire(ctx, full, ttl)
			}
			cancel()
			return val
		}
		cancel()
		s.markRedisDown(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if e, ok := s.local[full]; ok && !e.isExpired(now) {
		n, _ := strconv.ParseInt(e.value, 10, 64)
		n++
		s.local[full] = localEntry{strconv.FormatInt(n, 10), e.expiresAt}
		return n
	}
	s.local[full] = localEntry{"1", now.Add(ttl)}
	return 1
}

// SetIfAbsent fija la llave solo si no existe. Útil para enfriamientos.
func (s *Store) SetIfAbsent(key string, ttl time.Duration) bool {
	full := keyPrefix + key
	if s.redisReady() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		ok, err := s.rdb.SetNX(ctx, full, "1", ttl).Result()
		cancel()
		if err == nil {
			return ok
		}
		s.markRedisDown(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if e, ok := s.local[full]; ok && !e.isExpired(now) {
		return false
	}
	s.local[full] = localEntry{"1", now.Add(ttl)}
	return true
}

// Get lee una llave.
func (s *Store) Get(key string) (string, bool) {
	full := keyPrefix + key
	if s.redisReady() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		val, err := s.rdb.Get(ctx, full).Result()
		cancel()
		if err == nil {
			return val, true
		}
		if errors.Is(err, redis.Nil) {
			return "", false
		}
		s.markRedisDown(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.local[full]; ok && !e.isExpired(s.now()) {
		return e.value, true
	}
	return "", false
}

// Put guarda una llave con su TTL.
func (s *Store) Put(key, value string, ttl time.Duration) {
	full := keyPrefix + key
	if s.redisReady() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		err := s.rdb.Set(ctx, full, value, ttl).Err()
		cancel()
		if err == nil {
			return
		}
		s.markRedisDown(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.local[full] = localEntry{value, s.now().Add(ttl)}
}

// Take lee y borra en una sola operación: sirve para códigos de un solo uso.
func (s *Store) Take(key string) (string, bool) {
	full := keyPrefix + key
	if s.redisReady() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		val, err := s.rdb.GetDel(ctx, full).Result()
		cancel()
		if err == nil {
			return val, true
		}
		if errors.Is(err, redis.Nil) {
			return "", false
		}
		s.markRedisDown(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.local[full]
	if !ok {
		return "", false
	}
	delete(s.local, full)
	if e.isExpired(s.now()) {
		return "", false
	}
	return e.value, true
}

// RedisAvailable hace ping a Redis (independiente del backoff).
func (s *Store) RedisAvailable() bool {
	if s.rdb == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	return s.rdb.Ping(ctx).Err() == nil
}

// PurgeExpired limpia las entradas locales vencidas; se llama periódicamente.
func (s *Store) PurgeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, e := range s.local {
		if e.isExpired(now) {
			delete(s.local, k)
		}
	}
}

// Close cierra el cliente de Redis si existe.
func (s *Store) Close() error {
	if s.rdb == nil {
		return nil
	}
	return s.rdb.Close()
}

func (s *Store) redisReady() bool {
	if s.rdb == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.now().Before(s.retryAt)
}

func (s *Store) markRedisDown(err error) {
	s.mu.Lock()
	s.retryAt = s.now().Add(redisBackoff)
	s.mu.Unlock()
	slog.Warn("Redis no está disponible; se usa memoria local", "backoff_s", int(redisBackoff.Seconds()), "err", err)
}
