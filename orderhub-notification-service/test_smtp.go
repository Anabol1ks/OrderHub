package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	gopkgmail "gopkg.in/gomail.v2"
)

func main() {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found")
	}

	// Получаем настройки из переменных окружения
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := 465
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := os.Getenv("SMTP_FROM")

	fmt.Printf("SMTP Settings:\n")
	fmt.Printf("  Host: %s\n", smtpHost)
	fmt.Printf("  Port: %d\n", smtpPort)
	fmt.Printf("  User: %s\n", smtpUser)
	fmt.Printf("  From: %s\n", smtpFrom)
	fmt.Printf("  Password: %s\n", "***"+smtpPassword[len(smtpPassword)-4:])

	// Создаем простое письмо
	m := gopkgmail.NewMessage()
	m.SetHeader("From", smtpFrom)
	m.SetHeader("To", "grigorogannisyan.12@gmail.com")
	m.SetHeader("Subject", "Test Email from OrderHub - Direct SMTP Test")
	m.SetBody("text/plain", "Это тестовое письмо для проверки SMTP соединения.\n\nЕсли вы получили это письмо, значит SMTP настроен правильно!")

	// Создаем dialer и отправляем
	d := gopkgmail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPassword)
	d.SSL = true

	fmt.Println("\nОтправка тестового письма...")
	if err := d.DialAndSend(m); err != nil {
		fmt.Printf("❌ Ошибка отправки: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Println("✅ Письмо отправлено успешно!")
	}
}
