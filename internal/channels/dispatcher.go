package channels

import (
	"context"
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/notifications"
)

// Dispatcher reenvía cada notificación de la PWA a los canales externos. Corre
// aparte del flujo que la creó: un canal lento o caído no frena la lectura ni el
// comando.
type Dispatcher struct {
	channels     *Service
	cropChannels *CropChannelService
	sender       *Sender
	webBaseURL   string
}

// NewDispatcher crea el despachador.
func NewDispatcher(channels *Service, cropChannels *CropChannelService, sender *Sender, webBaseURL string) *Dispatcher {
	return &Dispatcher{channels: channels, cropChannels: cropChannels, sender: sender,
		webBaseURL: strings.TrimRight(webBaseURL, "/")}
}

// OnNotification reenvía una notificación recién creada. Corre en su goroutine.
func (d *Dispatcher) OnNotification(event notifications.CreatedEvent) {
	n := event.Notification
	ctx := context.Background()
	cropID := ""
	if n.CropID != nil {
		cropID = n.CropID.Hex()
	}
	url := d.webBaseURL + "/app"
	if cropID != "" {
		url = d.webBaseURL + "/app/crops/" + cropID
	}
	message := ChannelMessage{Type: n.Type, Title: n.Title, Body: n.Message, URL: url}
	for _, t := range AllTypes {
		if channel, ok := d.channels.AvailableChannel(t); ok {
			d.dispatch(ctx, channel, n, cropID, message)
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, channel NotificationChannel, n notifications.Notification, cropID string, message ChannelMessage) {
	t := channel.Type()
	link, _ := d.channels.Link(ctx, n.UserID.Hex(), t)
	if link != nil && !link.Enabled {
		link = nil
	}
	var settings *CropChannel
	if cropID != "" {
		settings, _ = d.cropChannels.Find(ctx, cropID, t)
	}
	if settings != nil && (!settings.Enabled || settings.Delivery == DeliveryDigest) {
		return
	}
	var events []notifications.Type
	switch {
	case settings != nil:
		events = settings.Events
	case link != nil:
		events = link.Events
	}
	if !containsType(events, n.Type) {
		return
	}
	if link != nil {
		d.sender.ToOwner(ctx, channel, link, message)
	}
	if settings != nil {
		d.sender.ToRecipients(ctx, channel, settings, message)
	}
}
