package core

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const projectExplorerCacheSchemaVersion = "v1"

type ProjectExplorerCacheConfig struct {
	Mode             string
	OperationTimeout time.Duration
	IndexTTL         time.Duration
	RowTTL           time.Duration
	LockTTL          time.Duration
	MaxValueBytes    int
}

type ProjectExplorerCacheObserver interface {
	ObserveProjectExplorerCache(layer, outcome string, count int)
	ObserveProjectExplorerCacheOperation(operation string, elapsed time.Duration)
	ObserveProjectExplorerCacheFallback(reason string)
	ObserveProjectExplorerCacheMismatch()
}

type projectExplorerRuntime struct {
	mode     string
	cache    *redisProjectExplorerCache
	observer ProjectExplorerCacheObserver
	group    singleflight.Group
}

type redisProjectExplorerCache struct {
	client           *redis.Client
	timeout          time.Duration
	indexTTL         time.Duration
	rowTTL           time.Duration
	lockTTL          time.Duration
	maxValueBytes    int
	consecutiveError atomic.Int32
	openUntil        atomic.Int64
}

var (
	errProjectExplorerCacheCircuitOpen   = errors.New("project explorer cache circuit is open")
	errProjectExplorerCacheScopeMismatch = errors.New("project explorer cache scope mismatch")
	errProjectExplorerCacheRevisionMoved = errors.New("project explorer cache revision changed during read")
)

func (s *Service) ConfigureProjectExplorerCache(client *redis.Client, config ProjectExplorerCacheConfig, observer ProjectExplorerCacheObserver) {
	mode := normalizeProjectExplorerCacheMode(config.Mode)
	runtime := &projectExplorerRuntime{mode: mode, observer: observer}
	if client != nil && mode != "off" {
		if config.OperationTimeout <= 0 {
			config.OperationTimeout = 200 * time.Millisecond
		}
		if config.IndexTTL <= 0 {
			config.IndexTTL = 5 * time.Minute
		}
		if config.RowTTL <= 0 {
			config.RowTTL = 10 * time.Minute
		}
		if config.LockTTL <= 0 {
			config.LockTTL = 2 * time.Second
		}
		if config.MaxValueBytes <= 0 {
			config.MaxValueBytes = 256 * 1024
		}
		runtime.cache = &redisProjectExplorerCache{
			client:        client,
			timeout:       config.OperationTimeout,
			indexTTL:      config.IndexTTL,
			rowTTL:        config.RowTTL,
			lockTTL:       config.LockTTL,
			maxValueBytes: config.MaxValueBytes,
		}
	}
	s.projectExplorer = runtime
}

func normalizeProjectExplorerCacheMode(mode string) string {
	switch mode {
	case "shadow", "serve":
		return mode
	default:
		return "off"
	}
}

func (s *Service) projectExplorerPage(ctx context.Context, actorID string, request ProjectExplorerRequest) (ProjectExplorerPage, error) {
	runtime := s.projectExplorer
	if runtime == nil || runtime.mode == "off" || runtime.cache == nil {
		if runtime != nil {
			runtime.fallback("disabled")
		}
		return s.projectExplorerDatabasePage(ctx, actorID, request)
	}
	if runtime.mode == "shadow" {
		before, err := s.repo.ProjectExplorerRevisionSnapshot(ctx, actorID)
		if err != nil {
			runtime.fallback("revision_error")
			return s.projectExplorerDatabasePage(ctx, actorID, request)
		}
		authoritative, err := s.projectExplorerDatabasePage(ctx, actorID, request)
		if err != nil {
			return ProjectExplorerPage{}, err
		}
		cached, cacheErr := s.projectExplorerCachedPage(ctx, actorID, request, before)
		after, revisionErr := s.repo.ProjectExplorerRevisionSnapshot(ctx, actorID)
		if cacheErr == nil && revisionErr == nil && before == after && !projectExplorerPagesEqual(authoritative, cached) {
			runtime.mismatch()
			log.Printf("event=project_explorer_cache_shadow_mismatch")
		}
		return authoritative, nil
	}

	revision, err := s.repo.ProjectExplorerRevisionSnapshot(ctx, actorID)
	if err != nil {
		runtime.fallback("revision_error")
		return s.projectExplorerDatabasePage(ctx, actorID, request)
	}
	page, err := s.projectExplorerCachedPage(ctx, actorID, request, revision)
	if err != nil {
		runtime.fallback(cacheFallbackReason(err))
		return s.projectExplorerDatabasePage(ctx, actorID, request)
	}
	return page, nil
}

func (s *Service) projectExplorerCachedPage(ctx context.Context, actorID string, request ProjectExplorerRequest, revision projectExplorerRevisionSnapshot) (ProjectExplorerPage, error) {
	runtime := s.projectExplorer
	indexKey := fmt.Sprintf("consflow:project-explorer:%s:index:%d:%s:%d:%s", projectExplorerCacheSchemaVersion, revision.CatalogVersion, actorID, revision.ActorVersion, request.cacheHash())
	startedAt := time.Now()
	index, hit, err := runtime.cache.getIndex(ctx, indexKey)
	runtime.operation("get_index", time.Since(startedAt))
	if err != nil {
		runtime.request("index", "error", 1)
		return ProjectExplorerPage{}, err
	}
	if hit {
		runtime.request("index", "hit", 1)
	} else {
		runtime.request("index", "miss", 1)
		loaded, loadErr, _ := runtime.group.Do(indexKey, func() (any, error) {
			if retry, retryHit, retryErr := runtime.cache.getIndex(ctx, indexKey); retryErr == nil && retryHit {
				return retry, nil
			}
			lockToken := uuid.NewString()
			locked, lockErr := runtime.cache.acquireLock(ctx, indexKey+":lock", lockToken)
			if lockErr != nil {
				return nil, lockErr
			}
			if !locked {
				if !waitProjectExplorerCache(ctx, 50*time.Millisecond) {
					return nil, ctx.Err()
				}
				if retry, retryHit, retryErr := runtime.cache.getIndex(ctx, indexKey); retryErr == nil && retryHit {
					return retry, nil
				}
			}
			if locked {
				defer runtime.cache.releaseLock(context.Background(), indexKey+":lock", lockToken)
			}
			fresh, dbErr := s.repo.ProjectExplorerIndex(ctx, actorID, request)
			if dbErr != nil {
				return nil, dbErr
			}
			if setErr := runtime.cache.setIndex(ctx, indexKey, fresh); setErr != nil {
				runtime.fallback(cacheFallbackReason(setErr))
			}
			return fresh, nil
		})
		if loadErr != nil {
			return ProjectExplorerPage{}, loadErr
		}
		index = loaded.(projectExplorerIndex)
	}

	versions, err := s.repo.ProjectExplorerProjectVersions(ctx, actorID, revision, index.IDs)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	if len(versions) != len(index.IDs) {
		runtime.cache.deleteBestEffort(indexKey)
		return ProjectExplorerPage{}, errProjectExplorerCacheScopeMismatch
	}
	rowKeys := make(map[string]string, len(index.IDs))
	for _, projectID := range index.IDs {
		rowKeys[projectID] = fmt.Sprintf("consflow:project-explorer:%s:row:%s:%d", projectExplorerCacheSchemaVersion, projectID, versions[projectID])
	}
	startedAt = time.Now()
	rows, misses, err := runtime.cache.getRows(ctx, rowKeys)
	runtime.operation("mget_rows", time.Since(startedAt))
	if err != nil {
		runtime.request("row", "error", len(rowKeys))
		return ProjectExplorerPage{}, err
	}
	runtime.request("row", "hit", len(rowKeys)-len(misses))
	runtime.request("row", "miss", len(misses))
	if len(misses) > 0 {
		missKey := "project-explorer-rows:"
		for _, projectID := range misses {
			missKey += projectID + ":" + fmt.Sprint(versions[projectID]) + ","
		}
		loaded, loadErr, _ := runtime.group.Do(missKey, func() (any, error) {
			return s.repo.ProjectExplorerRows(ctx, misses)
		})
		if loadErr != nil {
			return ProjectExplorerPage{}, loadErr
		}
		freshRows := loaded.(map[string]Record)
		for projectID, row := range freshRows {
			rows[projectID] = row
			if setErr := runtime.cache.setRow(ctx, rowKeys[projectID], row); setErr != nil {
				runtime.fallback(cacheFallbackReason(setErr))
			}
		}
	}
	page := assembleProjectExplorerPage(index, rows, request)
	currentRevision, err := s.repo.ProjectExplorerRevisionSnapshot(ctx, actorID)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	if currentRevision != revision {
		return ProjectExplorerPage{}, errProjectExplorerCacheRevisionMoved
	}
	currentVersions, err := s.repo.ProjectExplorerProjectVersions(ctx, actorID, currentRevision, index.IDs)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	if !projectExplorerVersionsEqual(versions, currentVersions) {
		return ProjectExplorerPage{}, errProjectExplorerCacheRevisionMoved
	}
	return page, nil
}

func (c *redisProjectExplorerCache) getIndex(ctx context.Context, key string) (projectExplorerIndex, bool, error) {
	payload, hit, err := c.get(ctx, key)
	if err != nil || !hit {
		return projectExplorerIndex{}, hit, err
	}
	var index projectExplorerIndex
	if json.Unmarshal(payload, &index) != nil || index.IDs == nil || index.Statuses == nil || index.Total < 0 {
		c.deleteBestEffort(key)
		return projectExplorerIndex{}, false, fmt.Errorf("decode project explorer index cache")
	}
	return index, true, nil
}

func (c *redisProjectExplorerCache) setIndex(ctx context.Context, key string, index projectExplorerIndex) error {
	return c.set(ctx, key, index, c.indexTTL)
}

func (c *redisProjectExplorerCache) getRows(ctx context.Context, keys map[string]string) (map[string]Record, []string, error) {
	rows := make(map[string]Record, len(keys))
	misses := make([]string, 0)
	if len(keys) == 0 {
		return rows, misses, nil
	}
	if err := c.available(); err != nil {
		return nil, nil, err
	}
	projectIDs := make([]string, 0, len(keys))
	for projectID := range keys {
		projectIDs = append(projectIDs, projectID)
	}
	sort.Strings(projectIDs)
	redisKeys := make([]string, 0, len(keys))
	for _, projectID := range projectIDs {
		redisKeys = append(redisKeys, keys[projectID])
	}
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	values, err := c.client.MGet(opCtx, redisKeys...).Result()
	if err != nil {
		c.markError()
		return nil, nil, err
	}
	c.markSuccess()
	for index, value := range values {
		projectID := projectIDs[index]
		if value == nil {
			misses = append(misses, projectID)
			continue
		}
		payload := []byte(fmt.Sprint(value))
		if len(payload) > c.maxValueBytes {
			c.deleteBestEffort(redisKeys[index])
			misses = append(misses, projectID)
			continue
		}
		var row Record
		if json.Unmarshal(payload, &row) != nil || cleanID(row["id"]) != projectID {
			c.deleteBestEffort(redisKeys[index])
			misses = append(misses, projectID)
			continue
		}
		rows[projectID] = row
	}
	return rows, misses, nil
}

func (c *redisProjectExplorerCache) setRow(ctx context.Context, key string, row Record) error {
	return c.set(ctx, key, row, c.rowTTL)
}

func (c *redisProjectExplorerCache) get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := c.available(); err != nil {
		return nil, false, err
	}
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	payload, err := c.client.Get(opCtx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		c.markSuccess()
		return nil, false, nil
	}
	if err != nil {
		c.markError()
		return nil, false, err
	}
	c.markSuccess()
	if len(payload) > c.maxValueBytes {
		c.deleteBestEffort(key)
		return nil, false, fmt.Errorf("project explorer cache value exceeds limit")
	}
	return payload, true, nil
}

func (c *redisProjectExplorerCache) set(ctx context.Context, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(payload) > c.maxValueBytes {
		return fmt.Errorf("project explorer cache value exceeds limit")
	}
	if err := c.available(); err != nil {
		return err
	}
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	err = c.client.Set(opCtx, key, payload, jitteredTTL(key, ttl)).Err()
	if err != nil {
		c.markError()
		return err
	}
	c.markSuccess()
	return nil
}

func (c *redisProjectExplorerCache) acquireLock(ctx context.Context, key, token string) (bool, error) {
	if err := c.available(); err != nil {
		return false, err
	}
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	locked, err := c.client.SetNX(opCtx, key, token, c.lockTTL).Result()
	if err != nil {
		c.markError()
		return false, err
	}
	c.markSuccess()
	return locked, nil
}

func (c *redisProjectExplorerCache) releaseLock(ctx context.Context, key, token string) {
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_ = c.client.Eval(opCtx, `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`, []string{key}, token).Err()
}

func (c *redisProjectExplorerCache) deleteBestEffort(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	_ = c.client.Del(ctx, key).Err()
}

func (c *redisProjectExplorerCache) available() error {
	if time.Now().UnixNano() < c.openUntil.Load() {
		return errProjectExplorerCacheCircuitOpen
	}
	return nil
}

func (c *redisProjectExplorerCache) markError() {
	if c.consecutiveError.Add(1) >= 3 {
		c.openUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
	}
}

func (c *redisProjectExplorerCache) markSuccess() {
	c.consecutiveError.Store(0)
	c.openUntil.Store(0)
}

func jitteredTTL(key string, ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Minute
	}
	sum := sha256.Sum256([]byte(key))
	window := ttl / 5
	if window <= 0 {
		return ttl
	}
	return ttl + time.Duration(binary.BigEndian.Uint64(sum[:8])%uint64(window))
}

func projectExplorerPagesEqual(left, right ProjectExplorerPage) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func waitProjectExplorerCache(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func cacheFallbackReason(err error) string {
	if errors.Is(err, errProjectExplorerCacheCircuitOpen) {
		return "breaker_open"
	}
	if errors.Is(err, errProjectExplorerCacheScopeMismatch) {
		return "scope_mismatch"
	}
	if errors.Is(err, errProjectExplorerCacheRevisionMoved) {
		return "revision_changed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "redis_error"
}

func projectExplorerVersionsEqual(left, right map[string]int64) bool {
	if len(left) != len(right) {
		return false
	}
	for projectID, version := range left {
		if right[projectID] != version {
			return false
		}
	}
	return true
}

func (r *projectExplorerRuntime) request(layer, outcome string, count int) {
	if count > 0 && r != nil && r.observer != nil {
		r.observer.ObserveProjectExplorerCache(layer, outcome, count)
	}
}

func (r *projectExplorerRuntime) operation(operation string, elapsed time.Duration) {
	if r != nil && r.observer != nil {
		r.observer.ObserveProjectExplorerCacheOperation(operation, elapsed)
	}
}

func (r *projectExplorerRuntime) fallback(reason string) {
	if r != nil && r.observer != nil {
		r.observer.ObserveProjectExplorerCacheFallback(reason)
	}
}

func (r *projectExplorerRuntime) mismatch() {
	if r != nil && r.observer != nil {
		r.observer.ObserveProjectExplorerCacheMismatch()
	}
}
