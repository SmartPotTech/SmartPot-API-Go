package mail

import (
	"fmt"
	"html"
)

// Paleta SmartPot para los correos (estilos en línea; los clientes de correo
// ignoran las hojas externas).
const (
	colorPrimary = "#067A52"
	colorDeep    = "#0B3D2B"
	colorBrand   = "#00B074"
	colorInk     = "#17261F"
	colorMuted   = "#5B6B63"
	colorSurface = "#F2F7F4"
)

type content struct {
	html string
	text string
}

func welcome(name, loginURL string) content {
	safeName := html.EscapeString(name)
	body := paragraph("Hola "+safeName+",") +
		paragraph("Tu cuenta de SmartPot está lista. Crea tu primer cultivo, real o virtual, y recibe "+
			"recomendaciones del asistente de IA para mantenerlo en su rango ideal.") +
		button("Ir a SmartPot", loginURL)
	text := "Hola " + name + ",\n\nTu cuenta de SmartPot está lista. Ingresa en: " + loginURL
	return content{html: layout("Bienvenido a SmartPot", body), text: text}
}

func passwordReset(name, resetURL string, minutes int64) content {
	safeName := html.EscapeString(name)
	body := paragraph("Hola "+safeName+",") +
		paragraph(fmt.Sprintf("Recibimos una solicitud para restablecer tu contraseña. El enlace vence en "+
			"%d minutos y solo se puede usar una vez.", minutes)) +
		button("Restablecer contraseña", resetURL) +
		small("Si no pediste este cambio, ignora este correo: tu contraseña sigue igual.")
	text := fmt.Sprintf("Hola %s,\n\nRestablece tu contraseña en: %s\nEl enlace vence en %d minutos. "+
		"Si no pediste el cambio, ignora este correo.", name, resetURL, minutes)
	return content{html: layout("Restablece tu contraseña", body), text: text}
}

func layout(title, body string) string {
	return "<!doctype html><html lang=\"es\"><body style=\"margin:0;background:" + colorSurface +
		";font-family:Segoe UI,Arial,sans-serif;color:" + colorInk + "\">" +
		"<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" style=\"padding:24px 0\">" +
		"<tr><td align=\"center\"><table role=\"presentation\" width=\"560\" cellpadding=\"0\" cellspacing=\"0\" " +
		"style=\"background:#FFFFFF;border-radius:14px;overflow:hidden\">" +
		"<tr><td style=\"background:" + colorDeep + ";padding:22px 28px;border-bottom:4px solid " + colorBrand + "\">" +
		"<span style=\"color:#FFFFFF;font-size:20px;font-weight:700;letter-spacing:.3px\">Smart<span style=\"color:" +
		colorBrand + "\">Pot</span></span></td></tr>" +
		"<tr><td style=\"padding:28px\"><h1 style=\"margin:0 0 16px;font-size:22px;color:" + colorPrimary + "\">" +
		html.EscapeString(title) + "</h1>" + body + "</td></tr>" +
		"<tr><td style=\"padding:16px 28px;background:" + colorSurface + ";font-size:12px;color:" + colorMuted + "\">" +
		"SmartPot · Monitoreo y automatización de cultivos hidropónicos</td></tr>" +
		"</table></td></tr></table></body></html>"
}

func paragraph(text string) string {
	return "<p style=\"margin:0 0 14px;font-size:15px;line-height:1.55\">" + text + "</p>"
}

func small(text string) string {
	return "<p style=\"margin:18px 0 0;font-size:12px;color:" + colorMuted + "\">" + text + "</p>"
}

func button(label, url string) string {
	safeURL := html.EscapeString(url)
	return "<p style=\"margin:22px 0\"><a href=\"" + safeURL + "\" style=\"background:" + colorPrimary +
		";color:#FFFFFF;text-decoration:none;padding:12px 22px;border-radius:10px;font-weight:600;" +
		"display:inline-block\">" + label + "</a></p>"
}
