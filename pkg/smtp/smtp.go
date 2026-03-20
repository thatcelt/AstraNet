package smtp

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

// Config представляет конфигурацию SMTP
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Service представляет SMTP сервис
type Service struct {
	config Config
}

// NewService создает новый SMTP сервис
func NewService(config Config) *Service {
	return &Service{
		config: config,
	}
}

// SendCaptchaEmail отправляет email с кодом верификации
func (s *Service) SendCaptchaEmail(to string, code string) error {
	subject := "Your Verification Code - Astranet"

	htmlBody := fmt.Sprintf(`
		<html>
			<body style="font-family: Arial, sans-serif; padding: 20px; background-color: #f5f5f5;">
				<div style="max-width: 400px; margin: 0 auto; background: #fff; border-radius: 8px; padding: 30px; box-shadow: 0 2px 10px rgba(0,0,0,0.1);">
					<h2 style="color: #333; margin-top: 0;">Email Verification</h2>
					<p style="color: #555;">Your verification code:</p>
					<div style="background: #f0f0f0; border-radius: 8px; padding: 15px; text-align: center; margin: 20px 0;">
						<span style="font-size: 32px; font-weight: bold; letter-spacing: 8px; color: #333;">%s</span>
					</div>
					<p style="color: #999; font-size: 13px;">This code will expire in 10 minutes.</p>
					<p style="color: #999; font-size: 13px;">If you didn't request this code, please ignore this email.</p>
				</div>
			</body>
		</html>
	`, code)

	return s.SendEmail(to, subject, htmlBody)
}

// SendEmail отправляет email
func (s *Service) SendEmail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n%s\r\n%s",
		s.config.From,
		to,
		subject,
		mime,
		body,
	))

	recipients := []string{to}
	err := smtp.SendMail(addr, auth, s.config.From, recipients, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendEmailWithImage отправляет email с встроенным изображением
func (s *Service) SendEmailWithImage(to, subject, htmlBody string, imageData []byte, imageName string) error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Граница для multipart сообщения
	boundary := "----=_NextPart_000_0000_01D0000.00000000"

	// Заголовки письма
	headers := make(map[string]string)
	headers["From"] = s.config.From
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = fmt.Sprintf("multipart/related; boundary=\"%s\"", boundary)

	// Формируем сообщение
	var msg strings.Builder

	// Добавляем заголовки
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")

	// HTML часть
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n\r\n")

	// Изображение как встроенное (inline)
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: image/png; name=\"" + imageName + "\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString("Content-ID: <captcha>\r\n")
	msg.WriteString("Content-Disposition: inline; filename=\"" + imageName + "\"\r\n\r\n")

	// Кодируем изображение в base64
	encoded := base64.StdEncoding.EncodeToString(imageData)
	// Разбиваем на строки по 76 символов (стандарт MIME)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msg.WriteString(encoded[i:end])
		msg.WriteString("\r\n")
	}

	// Закрывающая граница
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// Отправляем
	recipients := []string{to}
	err := smtp.SendMail(addr, auth, s.config.From, recipients, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("failed to send email with image: %w", err)
	}

	return nil
}

// ValidateEmail проверяет корректность email адреса
func ValidateEmail(email string) bool {
	if len(email) > 254 || len(email) < 5 {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if len(local) == 0 || len(local) > 64 {
		return false
	}
	if len(domain) == 0 || !strings.Contains(domain, ".") {
		return false
	}
	return true
}
