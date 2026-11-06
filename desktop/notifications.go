package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const importNotificationsKey = "import-notifications"

func (a *DesktopApp) NotifyImportFinished(title, body string) {
	if prefs, err := a.preferences.Read(); err == nil && prefs[importNotificationsKey] == "false" {
		return
	}
	if a.native.windowFocused() {
		return
	}
	if err := a.native.notify(title, body); err != nil {
		log.Println("Import notification not sent:", err)
	}
}

type notificationBackend interface {
	start() error
	authorized() (bool, error)
	requestAccess() (bool, error)
	deliver(options notifications.NotificationOptions) error
}

type wailsNotifications struct {
	service *notifications.NotificationService
}

func (w wailsNotifications) start() error {
	return w.service.ServiceStartup(context.Background(), application.ServiceOptions{})
}

func (w wailsNotifications) authorized() (bool, error) {
	return w.service.CheckNotificationAuthorization()
}

func (w wailsNotifications) requestAccess() (bool, error) {
	return w.service.RequestNotificationAuthorization()
}

func (w wailsNotifications) deliver(options notifications.NotificationOptions) error {
	return w.service.SendNotification(options)
}

type lazyNotifier struct {
	mu       sync.Mutex
	backend  notificationBackend
	started  bool
	startErr error
	denied   bool
}

// Not registered as a Wails service: Wails v3.1.0's macOS notifier fails
// ServiceStartup without a bundle identifier, and application.Run aborts on
// any failing service.
func (n *lazyNotifier) send(title, body string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.denied {
		return nil
	}
	if !n.started {
		n.started = true
		n.startErr = n.backend.start()
	}
	if n.startErr != nil {
		return n.startErr
	}
	granted, err := n.backend.authorized()
	if err != nil {
		return err
	}
	if !granted {
		granted, err = n.backend.requestAccess()
		if err != nil {
			return err
		}
		if !granted {
			n.denied = true
			return nil
		}
	}
	return n.backend.deliver(notifications.NotificationOptions{
		ID:    "import-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Title: title,
		Body:  body,
	})
}
