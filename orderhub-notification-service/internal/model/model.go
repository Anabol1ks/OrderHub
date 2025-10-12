package model

type EmailNotification struct {
	To       string
	Subject  string
	Template string         // имя шаблона (например, "verify_email")
	Data     map[string]any // данные для шаблона
}
