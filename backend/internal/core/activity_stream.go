package core

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type ActivityStreamEvent struct {
	Type       string `json:"type"`
	ProjectID  string `json:"project_id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
	ActivityID string `json:"activity_id,omitempty"`
	ScopeType  string `json:"scope_type,omitempty"`
	ScopeID    string `json:"scope_id,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	At         string `json:"at"`
}

type ActivityStreamHub struct {
	mu                    sync.RWMutex
	nextID                int
	subscribers           map[int]chan ActivityStreamEvent
	closed                bool
	redisClient           *redis.Client
	redisTimeout          time.Duration
	brokerChannel         string
	instanceID            string
	brokerCancel          context.CancelFunc
	brokerDone            chan struct{}
	closeOnce             sync.Once
	redisUnavailableUntil atomic.Int64
	brokerUnavailable     atomic.Bool
}

func NewActivityStreamHub() *ActivityStreamHub {
	return &ActivityStreamHub{
		subscribers: map[int]chan ActivityStreamEvent{},
		instanceID:  uuid.NewString(),
	}
}

func (h *ActivityStreamHub) Subscribe() (int, <-chan ActivityStreamEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		ch := make(chan ActivityStreamEvent)
		close(ch)
		return 0, ch
	}
	h.nextID++
	id := h.nextID
	ch := make(chan ActivityStreamEvent, 16)
	h.subscribers[id] = ch
	return id, ch
}

func (h *ActivityStreamHub) Unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.subscribers[id]
	if !ok {
		return
	}
	delete(h.subscribers, id)
	close(ch)
}

func (h *ActivityStreamHub) Publish(event ActivityStreamEvent) {
	if h == nil {
		return
	}
	if event.Type == "" {
		event.Type = "activity_changed"
	}
	if event.At == "" {
		event.At = time.Now().UTC().Format(time.RFC3339)
	}
	h.broadcast(event)

	h.mu.RLock()
	client := h.redisClient
	channel := h.brokerChannel
	instanceID := h.instanceID
	timeout := h.redisTimeout
	closed := h.closed
	h.mu.RUnlock()
	if closed || client == nil || channel == "" {
		return
	}
	now := time.Now()
	if now.UnixNano() < h.redisUnavailableUntil.Load() {
		return
	}
	payload, err := json.Marshal(activityBrokerMessage{Source: instanceID, Event: event})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Publish(ctx, channel, payload).Err(); err != nil {
		h.redisUnavailableUntil.Store(now.Add(5 * time.Second).UnixNano())
		return
	}
	h.redisUnavailableUntil.Store(0)
}

func (h *ActivityStreamHub) PublishDurable(ctx context.Context, event ActivityStreamEvent) error {
	if h == nil {
		return errors.New("activity stream hub is unavailable")
	}
	if event.Type == "" {
		event.Type = "activity_changed"
	}
	if event.At == "" {
		event.At = time.Now().UTC().Format(time.RFC3339)
	}
	h.broadcast(event)

	h.mu.RLock()
	client := h.redisClient
	channel := h.brokerChannel
	instanceID := h.instanceID
	timeout := h.redisTimeout
	closed := h.closed
	h.mu.RUnlock()
	if closed || client == nil || channel == "" {
		return errors.New("activity stream redis broker is unavailable")
	}
	payload, err := json.Marshal(activityBrokerMessage{Source: instanceID, Event: event})
	if err != nil {
		return err
	}
	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Publish(publishCtx, channel, payload).Err(); err != nil {
		return err
	}
	return nil
}

func (h *ActivityStreamHub) broadcast(event ActivityStreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for _, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

type activityBrokerMessage struct {
	Source string              `json:"source"`
	Event  ActivityStreamEvent `json:"event"`
}

func (h *ActivityStreamHub) ConfigureRedis(client *redis.Client, channel string, operationTimeout time.Duration) {
	if h == nil || client == nil || channel == "" {
		return
	}
	if operationTimeout <= 0 {
		operationTimeout = 250 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.redisClient = client
	h.redisTimeout = operationTimeout
	h.brokerChannel = channel
	h.brokerCancel = cancel
	h.brokerDone = make(chan struct{})
	done := h.brokerDone
	h.mu.Unlock()
	go h.consumeRedis(ctx, client, channel, operationTimeout, done)
}

func (h *ActivityStreamHub) consumeRedis(ctx context.Context, client *redis.Client, channel string, receiveTimeout time.Duration, done chan struct{}) {
	defer close(done)
	for ctx.Err() == nil {
		pubsub := client.Subscribe(ctx, channel)
		if _, err := pubsub.ReceiveTimeout(ctx, receiveTimeout); err != nil {
			_ = pubsub.Close()
			if ctx.Err() != nil {
				return
			}
			h.markBrokerDisconnected(err)
			if !waitForBrokerRetry(ctx) {
				return
			}
			continue
		}
		h.markBrokerRecovered()
		messages := pubsub.Channel()
		consume := true
		for consume {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case message, ok := <-messages:
				if !ok {
					consume = false
					continue
				}
				var payload activityBrokerMessage
				if json.Unmarshal([]byte(message.Payload), &payload) != nil || payload.Source == h.instanceID {
					continue
				}
				h.broadcast(payload.Event)
			}
		}
		_ = pubsub.Close()
		if ctx.Err() == nil {
			h.markBrokerDisconnected(fmt.Errorf("subscription channel closed"))
		}
		if !waitForBrokerRetry(ctx) {
			return
		}
	}
}

func (h *ActivityStreamHub) markBrokerDisconnected(err error) {
	if !h.brokerUnavailable.Swap(true) {
		log.Printf("activity stream redis broker disconnected; local delivery remains active: %v", err)
	}
}

func (h *ActivityStreamHub) markBrokerRecovered() {
	if h.brokerUnavailable.Swap(false) {
		log.Printf("activity stream redis broker recovered; cross-replica delivery restored")
	}
}

func waitForBrokerRetry(ctx context.Context) bool {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (h *ActivityStreamHub) Close() {
	if h == nil {
		return
	}
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		cancel := h.brokerCancel
		done := h.brokerDone
		for id, ch := range h.subscribers {
			delete(h.subscribers, id)
			close(ch)
		}
		h.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if done != nil {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	})
}

type ActivityStreamCapacity struct {
	mu          sync.Mutex
	maxTotal    int
	maxPerIP    int
	maxPerActor int
	total       int
	byIP        map[string]int
	byActor     map[string]int
}

func NewActivityStreamCapacity(maxTotal, maxPerIP, maxPerActor int) *ActivityStreamCapacity {
	if maxTotal <= 0 {
		maxTotal = 500
	}
	if maxPerIP <= 0 {
		maxPerIP = 20
	}
	if maxPerActor <= 0 {
		maxPerActor = 2
	}
	return &ActivityStreamCapacity{
		maxTotal:    maxTotal,
		maxPerIP:    maxPerIP,
		maxPerActor: maxPerActor,
		byIP:        map[string]int{},
		byActor:     map[string]int{},
	}
}

func (l *ActivityStreamCapacity) Acquire(ip, actorID string) (func(), bool) {
	if l == nil {
		return func() {}, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.maxTotal || l.byIP[ip] >= l.maxPerIP || l.byActor[actorID] >= l.maxPerActor {
		return nil, false
	}
	l.total++
	l.byIP[ip]++
	l.byActor[actorID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.total--
			decrementStreamCount(l.byIP, ip)
			decrementStreamCount(l.byActor, actorID)
		})
	}, true
}

func decrementStreamCount(values map[string]int, key string) {
	if values[key] <= 1 {
		delete(values, key)
		return
	}
	values[key]--
}

func (s *Service) ConfigureActivityStreaming(client *redis.Client, channel string, redisTimeout time.Duration, maxTotal, maxPerIP, maxPerActor int, visibilityTimeout, maxConnectionLifetime time.Duration) {
	if s.streamHub != nil {
		s.streamHub.ConfigureRedis(client, channel, redisTimeout)
	}
	s.streamCapacity = NewActivityStreamCapacity(maxTotal, maxPerIP, maxPerActor)
	if visibilityTimeout > 0 {
		s.streamVisibilityTimeout = visibilityTimeout
	}
	if maxConnectionLifetime > 0 {
		s.streamMaxConnectionLifetime = maxConnectionLifetime
	}
}

func (s *Service) CloseActivityStreaming() {
	if s != nil && s.streamHub != nil {
		s.streamHub.Close()
	}
}

func (s *Service) publishActivityEvent(event ActivityStreamEvent) {
	if s.streamHub == nil {
		return
	}
	s.streamHub.Publish(event)
}

func (s *Service) publishProjectProgressChanged(projectID string) {
	s.publishActivityEvent(ActivityStreamEvent{
		Type:      "project_progress_changed",
		ProjectID: cleanID(projectID),
	})
}

func (s *Service) publishWorkQueueChanged(projectID string) {
	s.publishActivityEvent(ActivityStreamEvent{
		Type:      "work_queue_changed",
		ProjectID: cleanID(projectID),
	})
}

func activityEventFromRecord(eventType string, activity Record, actorID string) ActivityStreamEvent {
	return ActivityStreamEvent{
		Type:       eventType,
		ProjectID:  cleanID(activity["project_id"]),
		ActivityID: cleanID(activity["id"]),
		ScopeType:  fmt.Sprint(activity["scope_type"]),
		ScopeID:    cleanID(activity["scope_id"]),
		ActorID:    actorID,
	}
}

func (s *Service) CanStreamProjectActivity(ctx context.Context, projectID, actorID string) (bool, error) {
	projectID = cleanID(projectID)
	if projectID == "" || !validActivityUUID(projectID) {
		return false, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	return s.repo.CanViewProjectOperations(ctx, projectID, actorID)
}

func (h *Handler) ProjectActivityStream(c *fiber.Ctx) error {
	release, ok := h.acquireActivityStream(c)
	if !ok {
		return activityStreamCapacityExceeded(c)
	}
	projectID := c.Params("id")
	userID := actorID(c)
	allowed, err := h.service.CanStreamProjectActivity(c.Context(), projectID, userID)
	if err != nil {
		release()
		return err
	}
	if !allowed {
		release()
		return forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	h.writeActivityStream(c, release, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		if event.ProjectID != projectID {
			return "", ActivityStreamEvent{}, false
		}
		if event.Type == "work_item_changed" {
			return "", ActivityStreamEvent{}, false
		}
		return event.Type, projectActivityInvalidation(event), true
	})
	return nil
}

func (h *Handler) ProjectProgressBoardStream(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	release, ok := h.acquireActivityStream(c)
	if !ok {
		return activityStreamCapacityExceeded(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.service.streamVisibilityTimeout)
	visibleProjects, err := h.service.repo.ProjectVisibleProjectIDs(ctx, actorID(c))
	cancel()
	if err != nil {
		release()
		return err
	}
	h.writeActivityStream(c, release, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		return projectProgressBoardStreamEvent(event, visibleProjects)
	})
	return nil
}

func (h *Handler) ProjectExplorerStream(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	release, ok := h.acquireActivityStream(c)
	if !ok {
		return activityStreamCapacityExceeded(c)
	}
	userID := actorID(c)
	ctx, cancel := context.WithTimeout(context.Background(), h.service.streamVisibilityTimeout)
	visibleProjects, err := h.service.repo.ProjectVisibleProjectIDs(ctx, userID)
	cancel()
	if err != nil {
		release()
		return err
	}
	h.writeActivityStream(c, release, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		return projectExplorerStreamEvent(event, userID, visibleProjects)
	})
	return nil
}

func projectExplorerStreamEvent(event ActivityStreamEvent, actorID string, visibleProjects map[string]struct{}) (string, ActivityStreamEvent, bool) {
	if event.Type != "project_explorer_changed" {
		return "", ActivityStreamEvent{}, false
	}
	switch event.ScopeType {
	case "actor":
		if event.ActorID != actorID {
			return "", ActivityStreamEvent{}, false
		}
	case "project":
		if _, visible := visibleProjects[event.ProjectID]; !visible {
			return "", ActivityStreamEvent{}, false
		}
	case "catalog":
	default:
		return "", ActivityStreamEvent{}, false
	}
	return "project_explorer_changed", ActivityStreamEvent{
		Type:      "project_explorer_changed",
		ProjectID: event.ProjectID,
		At:        event.At,
	}, true
}

func projectProgressBoardStreamEvent(event ActivityStreamEvent, visibleProjects map[string]struct{}) (string, ActivityStreamEvent, bool) {
	if event.Type != "project_progress_changed" && event.Type != "work_item_changed" {
		return "", ActivityStreamEvent{}, false
	}
	if event.ProjectID != "" {
		if _, visible := visibleProjects[event.ProjectID]; !visible {
			return "", ActivityStreamEvent{}, false
		}
	}
	return "project_progress_changed", ActivityStreamEvent{
		Type:      "project_progress_changed",
		ProjectID: event.ProjectID,
		At:        event.At,
	}, true
}

func projectActivityInvalidation(event ActivityStreamEvent) ActivityStreamEvent {
	return ActivityStreamEvent{
		Type:      event.Type,
		ProjectID: event.ProjectID,
		At:        event.At,
	}
}

func (h *Handler) MyActivityAttentionStream(c *fiber.Ctx) error {
	release, ok := h.acquireActivityStream(c)
	if !ok {
		return activityStreamCapacityExceeded(c)
	}
	userID := actorID(c)
	ctx, cancel := context.WithTimeout(context.Background(), h.service.streamVisibilityTimeout)
	visibleProjects, err := h.service.repo.ProjectOperationsVisibleProjectIDs(ctx, userID)
	cancel()
	if err != nil {
		release()
		return err
	}
	h.writeActivityStream(c, release, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		if !activityAttentionEventVisible(event, visibleProjects) {
			return "", ActivityStreamEvent{}, false
		}
		eventName, output := myActivityAttentionStreamEvent(event)
		return eventName, output, true
	})
	return nil
}

func activityAttentionEventVisible(event ActivityStreamEvent, visibleProjects map[string]struct{}) bool {
	if event.ProjectID == "" {
		return true
	}
	_, visible := visibleProjects[event.ProjectID]
	return visible
}

func myActivityAttentionStreamEvent(event ActivityStreamEvent) (string, ActivityStreamEvent) {
	if statusIn(event.Type, "work_item_changed", "work_queue_changed") {
		return "work_queue_changed", ActivityStreamEvent{
			Type:      "work_queue_changed",
			ProjectID: event.ProjectID,
			At:        time.Now().UTC().Format(time.RFC3339),
		}
	}
	return "attention_changed", ActivityStreamEvent{
		Type:      "attention_changed",
		ProjectID: event.ProjectID,
		At:        time.Now().UTC().Format(time.RFC3339),
	}
}

func (h *Handler) acquireActivityStream(c *fiber.Ctx) (func(), bool) {
	return h.service.streamCapacity.Acquire(c.IP(), actorID(c))
}

func activityStreamCapacityExceeded(c *fiber.Ctx) error {
	const retrySeconds = 5
	c.Set("Retry-After", fmt.Sprint(retrySeconds))
	return response.Fail(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "Đã đạt giới hạn kết nối cập nhật trực tiếp", Record{"retry_after_seconds": retrySeconds, "request_class": "sse"})
}

func (h *Handler) writeActivityStream(c *fiber.Ctx, release func(), filter func(ActivityStreamEvent) (string, ActivityStreamEvent, bool)) {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache, no-transform")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer release()
		subID, events := h.service.streamHub.Subscribe()
		defer h.service.streamHub.Unsubscribe(subID)
		if err := writeSSE(w, "connected", ActivityStreamEvent{Type: "connected", At: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			return
		}
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		lifetime := time.NewTimer(jitteredSSEConnectionLifetime(h.service.streamMaxConnectionLifetime, cryptoRandomOffset))
		defer lifetime.Stop()
		for {
			select {
			case <-lifetime.C:
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				eventName, out, matched := filter(event)
				if !matched {
					continue
				}
				if err := writeSSE(w, eventName, out); err != nil {
					return
				}
			case <-heartbeat.C:
				if err := writeSSE(w, "heartbeat", ActivityStreamEvent{Type: "heartbeat", At: time.Now().UTC().Format(time.RFC3339)}); err != nil {
					return
				}
			}
		}
	})
}

type sseRandomOffset func(maxExclusive uint64) (uint64, error)

func cryptoRandomOffset(maxExclusive uint64) (uint64, error) {
	value, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetUint64(maxExclusive))
	if err != nil {
		return 0, err
	}
	return value.Uint64(), nil
}

func jitteredSSEConnectionLifetime(maxLifetime time.Duration, randomOffset sseRandomOffset) time.Duration {
	if maxLifetime <= 0 {
		return maxLifetime
	}
	minimum := maxLifetime - maxLifetime/5
	width := uint64(maxLifetime-minimum) + 1
	if randomOffset == nil {
		return maxLifetime
	}
	offset, err := randomOffset(width)
	if err != nil || offset >= width {
		return maxLifetime
	}
	return minimum + time.Duration(offset)
}

func writeSSE(w *bufio.Writer, eventName string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, raw); err != nil {
		return err
	}
	return w.Flush()
}
