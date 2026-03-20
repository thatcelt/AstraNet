package service

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"time"
)

type Captcha struct {
	Code      string
	Email     string
	Image     []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

type CaptchaService struct {
	captchaStore map[string]*Captcha
}

func NewCaptchaService() *CaptchaService {
	return &CaptchaService{
		captchaStore: make(map[string]*Captcha),
	}
}

func (s *CaptchaService) GenerateCaptcha(email string) (*Captcha, error) {
	code, err := generateRandomCode(5)
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha code: %w", err)
	}

	// Генерируем изображение капчи
	imgBytes, err := generateCaptchaImage(code)
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha image: %w", err)
	}

	captcha := &Captcha{
		Code:      code,
		Email:     email,
		Image:     imgBytes,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute), // Капча действительна 10 минут
	}

	// Сохраняем капчу в хранилище
	s.captchaStore[email] = captcha

	return captcha, nil
}

// VerifyCaptcha проверяет правильность капчи
func (s *CaptchaService) VerifyCaptcha(email, code string) bool {
	captcha, exists := s.captchaStore[email]
	if !exists {
		return false
	}

	// Проверяем срок действия
	if time.Now().After(captcha.ExpiresAt) {
		delete(s.captchaStore, email)
		return false
	}

	// Проверяем код
	if captcha.Code != code {
		return false
	}

	// Удаляем использованную капчу
	delete(s.captchaStore, email)
	return true
}

// generateRandomCode генерирует случайный код из n цифр
func generateRandomCode(n int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, n)

	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}

	return string(code), nil
}

// generateCaptchaImage создает изображение капчи с помехами
func generateCaptchaImage(code string) ([]byte, error) {
	const (
		width  = 200
		height = 80
	)

	// Создаем новое изображение
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Заливаем фон случайным светлым цветом
	bgColor := color.RGBA{
		R: uint8(200 + randomInt(55)),
		G: uint8(200 + randomInt(55)),
		B: uint8(200 + randomInt(55)),
		A: 255,
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bgColor)
		}
	}

	// Добавляем шумовые линии (помехи)
	for i := 0; i < 8; i++ {
		drawNoiseLine(img, width, height)
	}

	// Добавляем случайные точки
	for i := 0; i < 100; i++ {
		x := randomInt(width)
		y := randomInt(height)
		noiseColor := color.RGBA{
			R: uint8(randomInt(256)),
			G: uint8(randomInt(256)),
			B: uint8(randomInt(256)),
			A: 255,
		}
		img.Set(x, y, noiseColor)
	}

	// Рисуем цифры
	spacing := width / (len(code) + 1)
	for i, digit := range code {
		x := spacing * (i + 1)
		y := height / 2

		// Случайное смещение для каждой цифры
		offsetX := randomInt(10) - 5
		offsetY := randomInt(10) - 5

		drawDigit(img, string(digit), x+offsetX, y+offsetY)
	}

	// Добавляем еще шумовых линий поверх цифр
	for i := 0; i < 3; i++ {
		drawNoiseLine(img, width, height)
	}

	// Конвертируем в PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// drawDigit рисует цифру на изображении (упрощенный вариант)
func drawDigit(img *image.RGBA, digit string, centerX, centerY int) {
	// Цвет текста - случайный темный цвет
	textColor := color.RGBA{
		R: uint8(randomInt(100)),
		G: uint8(randomInt(100)),
		B: uint8(randomInt(100)),
		A: 255,
	}

	// Простая матрица для рисования цифр (5x7 пикселей)
	patterns := map[string][]string{
		"0": {" ### ", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
		"1": {"  #  ", " ##  ", "  #  ", "  #  ", "  #  ", "  #  ", "#####"},
		"2": {" ### ", "#   #", "    #", "   # ", "  #  ", " #   ", "#####"},
		"3": {" ### ", "#   #", "    #", "  ## ", "    #", "#   #", " ### "},
		"4": {"   # ", "  ## ", " # # ", "#  # ", "#####", "   # ", "   # "},
		"5": {"#####", "#    ", "#### ", "    #", "    #", "#   #", " ### "},
		"6": {" ### ", "#    ", "#    ", "#### ", "#   #", "#   #", " ### "},
		"7": {"#####", "    #", "   # ", "  #  ", " #   ", " #   ", " #   "},
		"8": {" ### ", "#   #", "#   #", " ### ", "#   #", "#   #", " ### "},
		"9": {" ### ", "#   #", "#   #", " ####", "    #", "    #", " ### "},
	}

	pattern, ok := patterns[digit]
	if !ok {
		return
	}

	// Масштаб для увеличения цифр
	scale := 3
	// Случайный угол поворота (небольшой наклон)
	angle := float64(randomInt(30)-15) * math.Pi / 180

	for row, line := range pattern {
		for col, ch := range line {
			if ch == '#' {
				// Применяем масштаб и поворот
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						px := float64(col*scale + dx - len(line)*scale/2)
						py := float64(row*scale + dy - len(pattern)*scale/2)

						// Поворот
						rotX := px*math.Cos(angle) - py*math.Sin(angle)
						rotY := px*math.Sin(angle) + py*math.Cos(angle)

						finalX := centerX + int(rotX)
						finalY := centerY + int(rotY)

						if finalX >= 0 && finalX < img.Bounds().Dx() &&
							finalY >= 0 && finalY < img.Bounds().Dy() {
							img.Set(finalX, finalY, textColor)
						}
					}
				}
			}
		}
	}
}

// drawNoiseLine рисует случайную линию помехи
func drawNoiseLine(img *image.RGBA, width, height int) {
	lineColor := color.RGBA{
		R: uint8(randomInt(256)),
		G: uint8(randomInt(256)),
		B: uint8(randomInt(256)),
		A: 100, // Полупрозрачная
	}

	x1 := randomInt(width)
	y1 := randomInt(height)
	x2 := randomInt(width)
	y2 := randomInt(height)

	// Рисуем линию по алгоритму Брезенхема (упрощенно)
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)

	var sx, sy int
	if x1 < x2 {
		sx = 1
	} else {
		sx = -1
	}
	if y1 < y2 {
		sy = 1
	} else {
		sy = -1
	}

	err := dx - dy

	for {
		if x1 >= 0 && x1 < width && y1 >= 0 && y1 < height {
			img.Set(x1, y1, lineColor)
		}

		if x1 == x2 && y1 == y2 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// randomInt возвращает случайное число от 0 до max-1
func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// abs возвращает абсолютное значение
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
