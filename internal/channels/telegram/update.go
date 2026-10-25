// Package telegram implementa el canal de Telegram: el cliente del Bot API, el
// bot (comandos), la recepción de mensajes (polling o webhook) y el canal de
// notificación que usa el paquete channels.
package telegram

// Update es lo que interesa de una novedad del Bot API: un mensaje de texto.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message es un mensaje de un chat.
type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      *Chat  `json:"chat"`
	From      *User  `json:"from"`
	Text      string `json:"text"`
}

// Chat es el chat de un mensaje.
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// User es el remitente de un mensaje.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type updates struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}
