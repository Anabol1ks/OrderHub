package sender

import (
	"bytes"
	"fmt"
	"html/template"
	"notification-service/config"
	"notification-service/internal/model"
	"os"
	"path/filepath"
	"strings"

	gopkgmail "gopkg.in/gomail.v2"
)

type EmailSender struct {
	cfg *config.Config
}

func NewEmailSender(cfg *config.Config) *EmailSender {
	return &EmailSender{cfg: cfg}
}

func (s *EmailSender) SendEmail(n model.EmailNotification) error {
	htmlBody, err := s.renderHTML(n.Template, n.Data)
	if err != nil {
		return fmt.Errorf("render html: %w", err)
	}
	plainBody, err := s.renderPlain(n.Template, n.Data)
	if err != nil {
		return fmt.Errorf("render plain: %w", err)
	}

	m := gopkgmail.NewMessage()
	m.SetHeader("From", s.cfg.SMTPFrom)
	m.SetHeader("To", n.To)
	m.SetHeader("Subject", n.Subject)
	m.SetBody("text/plain", plainBody)
	m.AddAlternative("text/html", htmlBody)

	if strings.Contains(htmlBody, "cid:logo") {
		iconPath := filepath.Join(s.cfg.TMPLDir, "icon.png")
		if _, errStat := os.Stat(iconPath); errStat == nil {
			m.Embed(iconPath, gopkgmail.SetHeader(map[string][]string{"Content-ID": {"<logo>"}}))
		}
	}

	d := gopkgmail.NewDialer(s.cfg.SMTPHost, s.cfg.SMTPPort, s.cfg.SMTPUser, s.cfg.SMTPPassword)
	d.SSL = true
	return d.DialAndSend(m)
}

func (s *EmailSender) renderHTML(tmplName string, data map[string]any) (string, error) {
	path := filepath.Join(s.cfg.TMPLDir, tmplName+".html")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *EmailSender) renderPlain(tmplName string, data map[string]any) (string, error) {
	path := filepath.Join(s.cfg.TMPLDir, tmplName+".txt")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
