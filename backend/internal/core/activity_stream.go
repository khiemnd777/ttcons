package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ActivityStreamEvent struct {
	Type       string `json:"type"`
	ProjectID  string `json:"project_id,omitempty"`
	ActivityID string `json:"activity_id,omitempty"`
	ScopeType  string `json:"scope_type,omitempty"`
	ScopeID    string `json:"scope_id,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	At         string `json:"at"`
}

type ActivityStreamHub struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[int]chan ActivityStreamEvent
}

func NewActivityStreamHub() *ActivityStreamHub {
	return &ActivityStreamHub{subscribers: map[int]chan ActivityStreamEvent{}}
}

func (h *ActivityStreamHub) Subscribe() (int, <-chan ActivityStreamEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
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
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Service) publishActivityEvent(event ActivityStreamEvent) {
	if s.streamHub == nil {
		return
	}
	s.streamHub.Publish(event)
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
	projectID := c.Params("id")
	allowed, err := h.service.CanStreamProjectActivity(c.Context(), projectID, actorID(c))
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Bạn không có quyền xem hoạt động của công trình này")
	}
	h.writeActivityStream(c, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		if event.ProjectID != projectID {
			return "", ActivityStreamEvent{}, false
		}
		return event.Type, event, true
	})
	return nil
}

func (h *Handler) MyActivityAttentionStream(c *fiber.Ctx) error {
	userID := actorID(c)
	h.writeActivityStream(c, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) {
		if event.ProjectID != "" {
			allowed, err := h.service.CanStreamProjectActivity(context.Background(), event.ProjectID, userID)
			if err != nil || !allowed {
				return "", ActivityStreamEvent{}, false
			}
		}
		return "attention_changed", ActivityStreamEvent{
			Type:      "attention_changed",
			ProjectID: event.ProjectID,
			At:        time.Now().UTC().Format(time.RFC3339),
		}, true
	})
	return nil
}

func (h *Handler) writeActivityStream(c *fiber.Ctx, filter func(ActivityStreamEvent) (string, ActivityStreamEvent, bool)) {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache, no-transform")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		subID, events := h.service.streamHub.Subscribe()
		defer h.service.streamHub.Unsubscribe(subID)
		if err := writeSSE(w, "connected", ActivityStreamEvent{Type: "connected", At: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			return
		}
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
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
