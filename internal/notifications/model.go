// Package notifications gestiona las alertas del cultivo, del dispositivo y del
// asistente de IA que ve la PWA y que los canales externos reenvían.
package notifications

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Type es la clase de notificación.
type Type string

const (
	TypeInfo    Type = "INFO"
	TypeAlert   Type = "ALERT"
	TypeCommand Type = "COMMAND"
	TypeDevice  Type = "DEVICE"
	TypeAI      Type = "AI"
)

// Notification es el documento de la colección "notifications". cropId es
// opcional (las notificaciones de cuenta no tienen cultivo).
type Notification struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty"`
	UserID    primitive.ObjectID  `bson:"userId"`
	CropID    *primitive.ObjectID `bson:"cropId,omitempty"`
	Type      Type                `bson:"type"`
	Title     string              `bson:"title"`
	Message   string              `bson:"message"`
	Read      bool                `bson:"read"`
	CreatedAt time.Time           `bson:"createdAt"`
}

// CreatedEvent anuncia que se creó una notificación; los canales externos la
// reenvían si la persona lo pidió.
type CreatedEvent struct {
	Notification Notification
}

// Response es la vista pública de una notificación.
type Response struct {
	ID        string    `json:"id"`
	CropID    *string   `json:"cropId,omitempty"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToResponse mapea la entidad a su vista pública.
func ToResponse(n Notification) Response {
	var cropID *string
	if n.CropID != nil {
		hex := n.CropID.Hex()
		cropID = &hex
	}
	return Response{
		ID:        n.ID.Hex(),
		CropID:    cropID,
		Type:      string(n.Type),
		Title:     n.Title,
		Message:   n.Message,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
	}
}
