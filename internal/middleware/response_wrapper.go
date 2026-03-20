package middleware

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

func ResponseWrapper(c fiber.Ctx) error {
	// Пропускаем OPTIONS (CORS preflight), Swagger и другие служебные эндпоинты
	if c.Method() == "OPTIONS" {
		return c.Next()
	}
	path := c.Path()
	if strings.HasPrefix(path, "/swagger") {
		return c.Next()
	}

	start := time.Now()

	// Выполняем следующие хендлеры
	err := c.Next()

	duration := time.Since(start)

	// Форматируем длительность как "0.000s"
	durationStr := fmt.Sprintf("%.3fs", duration.Seconds())

	response := map[string]interface{}{
		"api:duration":  durationStr,
		"api:timestamp": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	// Если произошла ошибка (например, паника или явный возврат ошибки)
	if err != nil {
		code := fiber.StatusInternalServerError
		message := "Internal Server Error"
		aminoCode := 500

		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			message = e.Message
			// Маппинг HTTP кодов в Amino коды, если нужно
			if code == 400 {
				aminoCode = 100
			} else if code == 401 {
				aminoCode = 200
			} else if code == 404 {
				aminoCode = 216
			}
		}

		response["api:statuscode"] = aminoCode
		response["api:message"] = message

		c.Status(code)
		return c.JSON(response)
	}

	// Получаем текущий статус и тело ответа
	statusCode := c.Response().StatusCode()
	body := c.Response().Body()

	// По умолчанию для Amino успех - это 0
	aminoStatusCode := 0
	apiMessage := "OK"

	// Если в теле уже есть данные, пытаемся их распарсить
	var data map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err == nil {
			// Если в данных уже есть api:statuscode (например, от NewError), используем его
			if val, ok := data["api:statuscode"]; ok {
				if f, ok := val.(float64); ok {
					aminoStatusCode = int(f)
				}
			}
			if val, ok := data["api:message"]; ok {
				if s, ok := val.(string); ok {
					apiMessage = s
				}
			}

			// Копируем все остальные поля из данных в итоговый ответ
			for k, v := range data {
				if k != "api:statuscode" && k != "api:message" {
					response[k] = v
				}
			}
		} else {
			// Если это не JSON, кладем как есть в поле data
			response["data"] = string(body)
		}
	} else if statusCode < 400 {
		// Если тело пустое и это успех, добавляем дефолтный статус
		response["status"] = "ok"
	}

	// Если HTTP статус не 2xx, и мы еще не установили специфичный код Amino
	if statusCode >= 400 && aminoStatusCode == 0 {
		aminoStatusCode = statusCode // Или маппинг
		apiMessage = "Error"
	}

	response["api:statuscode"] = aminoStatusCode
	response["api:message"] = apiMessage

	c.Response().ResetBody()
	return c.JSON(response)
}
