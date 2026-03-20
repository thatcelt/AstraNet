package service

import (
	"fmt"

	"github.com/AugustLigh/GoMino/pkg/smtp"
)

// ValidationService отвечает за валидацию через email
type ValidationService struct {
	captchaService *CaptchaService
	smtpService    *smtp.Service
}

// NewValidationService создает новый сервис валидации
func NewValidationService(smtpConfig smtp.Config) *ValidationService {
	return &ValidationService{
		captchaService: NewCaptchaService(),
		smtpService:    smtp.NewService(smtpConfig),
	}
}

// SendValidationCode отправляет код валидации на указанный email
func (s *ValidationService) SendValidationCode(email string) error {
	// Генерируем капчу
	captcha, err := s.captchaService.GenerateCaptcha(email)
	if err != nil {
		return fmt.Errorf("failed to generate captcha: %w", err)
	}

	fmt.Printf("Generated captcha code: %s for email: %s\n", captcha.Code, email)

	// Отправляем код на email
	if err := s.smtpService.SendCaptchaEmail(email, captcha.Code); err != nil {
		return fmt.Errorf("failed to send captcha email: %w", err)
	}

	fmt.Printf("Captcha email sent successfully to: %s\n", email)
	return nil
}

// VerifyCode проверяет введенный пользователем код
func (s *ValidationService) VerifyCode(email, code string) bool {
	return s.captchaService.VerifyCaptcha(email, code)
}

// TestSendCaptcha - простой пример для тестирования отправки капчи
func TestSendCaptcha(email string, smtpConfig smtp.Config) error {
	validationService := NewValidationService(smtpConfig)

	fmt.Println("=== Testing Captcha Email ===")
	fmt.Printf("Sending captcha to: %s\n", email)

	if err := validationService.SendValidationCode(email); err != nil {
		return fmt.Errorf("test failed: %w", err)
	}

	fmt.Println("✅ Test completed successfully!")
	fmt.Println("Check your email for the captcha image.")

	return nil
}
