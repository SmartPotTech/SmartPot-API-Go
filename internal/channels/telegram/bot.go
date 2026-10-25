package telegram

import (
	"context"
	"strconv"
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/channels"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
)

const helpText = "Soy el asistente de <b>SmartPot</b> 🌱\n" +
	"Te aviso aquí cuando un cultivo necesita atención, cuando un cultivo se desconecta o cuando el " +
	"asistente actúa por su cuenta.\n\n" +
	"/estado — cómo están tus cultivos y los que te compartieron\n" +
	"/desvincular — dejar de recibir avisos en este chat\n" +
	"/ayuda — este mensaje\n\n" +
	"Para vincular este chat entra a SmartPot › Perfil › Aplicaciones y toca «Vincular Telegram». Si alguien " +
	"te comparte un cultivo, abre el enlace que te envíe."

const notLinked = "Este chat aún no está vinculado. Entra a SmartPot › Perfil › Aplicaciones y toca «Vincular Telegram»."

// Bot atiende la conversación con el bot (solo chats privados).
type Bot struct {
	channels     *channels.Service
	cropChannels *channels.CropChannelService
	crops        *crops.Service
	client       *Client
}

// NewBot crea el bot.
func NewBot(ch *channels.Service, cc *channels.CropChannelService, cropsSvc *crops.Service, client *Client) *Bot {
	return &Bot{channels: ch, cropChannels: cc, crops: cropsSvc, client: client}
}

// Handle atiende un mensaje entrante.
func (b *Bot) Handle(update Update) {
	msg := update.Message
	if msg == nil || msg.Chat == nil || msg.Text == "" {
		return
	}
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	if msg.Chat.Type != "private" {
		b.reply(chatID, "Escríbeme en un chat privado para vincular tus cultivos.")
		return
	}
	parts := strings.SplitN(strings.TrimSpace(msg.Text), " ", 2)
	command := strings.ToLower(parts[0])
	if i := strings.IndexByte(command, '@'); i >= 0 {
		command = command[:i]
	}
	argument := ""
	if len(parts) > 1 {
		argument = strings.TrimSpace(parts[1])
	}
	switch command {
	case "/start":
		b.start(chatID, argument, msg)
	case "/estado":
		b.reply(chatID, b.status(chatID))
	case "/desvincular":
		b.reply(chatID, b.unlink(chatID))
	default:
		b.reply(chatID, helpText)
	}
}

func (b *Bot) start(chatID, code string, msg *Message) {
	if code == "" {
		b.reply(chatID, helpText)
		return
	}
	ctx := context.Background()
	name := ""
	if msg.From != nil && msg.From.FirstName != "" {
		name = msg.From.FirstName
	} else if msg.Chat != nil {
		name = msg.Chat.FirstName
	}
	display := name
	if msg.Chat != nil && msg.Chat.Username != "" {
		display = "@" + msg.Chat.Username
	}
	greeting := ""
	if name != "" {
		greeting = ", " + escape(name)
	}
	if channels.IsShareCode(code) {
		crop, ok := b.cropChannels.AcceptShare(ctx, channels.Telegram, code, chatID, display)
		if ok && crop != nil {
			b.reply(chatID, "¡Listo"+greeting+"! Desde ahora recibirás aquí los avisos de «"+escape(crop.Name)+
				"». Escribe /estado para verlo y /desvincular para dejar de recibirlos.")
		} else {
			b.reply(chatID, "El enlace expiró, ya se usó o el cultivo ya se comparte con demasiados chats. Pide uno nuevo.")
		}
		return
	}
	link, ok := b.channels.CompleteLink(ctx, channels.Telegram, code, chatID, display)
	if ok && link != nil {
		b.reply(chatID, "¡Listo"+greeting+"! Este chat quedó vinculado a tu cuenta de SmartPot. Te avisaré de "+
			"alertas, desconexiones y acciones del asistente; puedes elegir cuáles en tu perfil.\n\nEscribe /estado para ver tus cultivos.")
	} else {
		b.reply(chatID, "El enlace expiró o ya se usó. Genera uno nuevo desde SmartPot › Perfil › Notificaciones.")
	}
}

func (b *Bot) unlink(chatID string) string {
	ctx := context.Background()
	linked, _ := b.channels.UnlinkAddress(ctx, channels.Telegram, chatID)
	shared, _ := b.cropChannels.Leave(ctx, channels.Telegram, chatID)
	if !linked && shared == 0 {
		return "Este chat no estaba vinculado a ninguna cuenta ni recibía avisos compartidos."
	}
	return "Listo: ya no recibirás avisos en este chat. Puedes volver a vincularlo desde tu perfil."
}

func (b *Bot) status(chatID string) string {
	ctx := context.Background()
	link, _ := b.channels.FindByAddress(ctx, channels.Telegram, chatID)
	shared, _ := b.cropChannels.SharedWith(ctx, channels.Telegram, chatID)
	if link == nil && len(shared) == 0 {
		return notLinked
	}
	var own []crops.Crop
	if link != nil {
		own, _ = b.crops.List(ctx, link.UserID.Hex())
	}
	if len(own) == 0 && len(shared) == 0 {
		return "Todavía no tienes cultivos. Crea el primero desde la app."
	}
	var text strings.Builder
	if len(own) > 0 {
		text.WriteString("🌱 <b>Tus cultivos</b>\n")
		describe(&text, own)
	}
	if len(shared) > 0 {
		if text.Len() > 0 {
			text.WriteString("\n\n")
		}
		text.WriteString("🤝 <b>Te compartieron</b>\n")
		describe(&text, shared)
	}
	return text.String()
}

func describe(text *strings.Builder, list []crops.Crop) {
	for i := range list {
		crop := &list[i]
		online := crop.Device != nil && crop.Device.Online
		text.WriteString("\n• <b>" + escape(crop.Name) + "</b>")
		if crop.IsVirtual() {
			text.WriteString(" (virtual)")
		}
		if online {
			text.WriteString(" — 🟢 en línea")
		} else {
			text.WriteString(" — ⚪ sin conexión")
		}
		if crop.Health != nil {
			text.WriteString(" · " + escape(crop.Health.Label) + " (" + strconv.Itoa(int(round(crop.Health.Index))) + "/100)")
		}
		if crop.AutomationEnabled {
			text.WriteString(" · 🤖 automático")
		}
	}
}

func (b *Bot) reply(chatID, html string) {
	_ = b.client.SendMessage(chatID, html, "", "")
}

func round(v float64) float64 {
	return float64(int64(v + 0.5))
}
