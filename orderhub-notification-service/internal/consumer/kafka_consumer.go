package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"notification-service/internal/model"
	"notification-service/internal/sender"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type EmailMessage struct {
	To       string         `json:"to"`
	Subject  string         `json:"subject"`
	Template string         `json:"template"`
	Data     map[string]any `json:"data"`
}

type KafkaEmailConsumer struct {
	reader      *kafka.Reader
	emailSender *sender.EmailSender
	log         *zap.Logger
}

func NewKafkaEmailConsumer(brokers []string, groupID, topic string, emailSender *sender.EmailSender, log *zap.Logger) *KafkaEmailConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           brokers,
		GroupID:           groupID,
		Topic:             topic,
		MinBytes:          10e3,
		MaxBytes:          10e6,
		CommitInterval:    time.Second,
		HeartbeatInterval: 3 * time.Second,
		SessionTimeout:    30 * time.Second,
	})
	return &KafkaEmailConsumer{reader: r, emailSender: emailSender, log: log}
}

func (c *KafkaEmailConsumer) Run(ctx context.Context) error {
	c.log.Info("kafka consumer started")
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.log.Error("read message", zap.Error(err))
			continue
		}
		var em EmailMessage
		if err := json.Unmarshal(m.Value, &em); err != nil {
			c.log.Error("unmarshal email message", zap.ByteString("value", m.Value), zap.Error(err))
			continue
		}
		if em.To == "" || em.Template == "" {
			c.log.Warn("invalid email message", zap.Any("msg", em))
			continue
		}
		if err = c.emailSender.SendEmail(model.EmailNotification{To: em.To, Subject: em.Subject, Template: em.Template, Data: em.Data}); err != nil {
			c.log.Error("send email failed", zap.String("to", em.To), zap.String("template", em.Template), zap.Error(err))
			continue
		}
		c.log.Info("email sent", zap.String("to", em.To), zap.String("template", em.Template))
	}
}

func (c *KafkaEmailConsumer) Close() error { return c.reader.Close() }
