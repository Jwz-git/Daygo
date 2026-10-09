package windows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// notificationDriver is the synchronous, one-shot native boundary. The OS owns
// delivery; this adapter has no product timer or reminder settings.
type notificationDriver interface {
	permission() (platform.PermissionState, error)
	schedule(platform.Notification) error
	cancel(string) error
	close()
}

type notificationClient struct {
	mu     sync.Mutex
	driver notificationDriver
}

func (n *notificationClient) available() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.driver != nil
}
func (n *notificationClient) permission(ctx context.Context) (platform.PermissionState, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if n.driver == nil {
		return "", platform.ErrCapabilityUnavailable
	}
	return n.driver.permission()
}
func (n *notificationClient) schedule(ctx context.Context, request platform.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.driver == nil {
		return platform.ErrCapabilityUnavailable
	}
	if !validNotificationText(request.ID, 4096) || request.ID == "" ||
		!validNotificationText(request.Title, 4096) || !validNotificationText(request.Body, 16384) {
		return fmt.Errorf("windows notification: invalid text")
	}
	if request.DeliverAt != nil && (request.DeliverAt.UTC().Year() < 1601 || request.DeliverAt.UTC().Year() > 9999) {
		return fmt.Errorf("windows notification: delivery time out of range")
	}
	if err := n.driver.schedule(request); err != nil {
		return err
	}
	return ctx.Err()
}
func (n *notificationClient) cancel(ctx context.Context, ids []string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.driver == nil {
		return platform.ErrCapabilityUnavailable
	}
	// Validate the whole batch before cancelling anything.
	for _, id := range ids {
		if id == "" || !validNotificationText(id, 4096) {
			return fmt.Errorf("windows notification: invalid identifier")
		}
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := n.driver.cancel(id); err != nil {
			return err
		}
	}
	return ctx.Err()
}
func (n *notificationClient) close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.driver != nil {
		n.driver.close()
		n.driver = nil
	}
}

// Windows limits ScheduledToastNotification.Id to 16 characters. The full ID
// also travels in the XML launch attribute, where the native side checks it
// before replacing/cancelling a matching token (including hash collisions).
func notificationToken(id string) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:8])
}

func validNotificationText(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if !(r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			return false
		}
	}
	return true
}
