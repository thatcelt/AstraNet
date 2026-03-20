package response

type ErrorResponse struct {
	StatusCode int    `json:"api:statuscode"`
	Message    string `json:"api:message"`
}

// Стандартные коды ошибок Amino
const (
	StatusOK                 = 0
	StatusInvalidRequest     = 100
	StatusNoPermission       = 103
	StatusInvalidCredentials = 200
	StatusAccountNotExist    = 216
	StatusEmailAlreadyTaken  = 213
	StatusInvalidEmail       = 213
	StatusInvalidPassword    = 218
	StatusInvalidVerifyCode  = 270
	StatusInvalidAPIKey      = 280
	StatusMissingAPIKey      = 281 // Request without API key
	StatusDPoPLoginRequired  = 282 // Login without DPoP keys
	StatusDPoPRefreshRequired = 283 // Token refresh without DPoP keys
	StatusDPoPBindingMissing = 284 // Token has no DPoP binding
	StatusUserBanned         = 105
	StatusBlockedByUser      = 150
	StatusReadMode           = 160 // User is in read mode
	StatusTooManyRequests    = 403
	StatusEmailRateLimited   = 291
	StatusWeakPassword       = 292
	StatusPasswordTooShort   = 293
	StatusPasswordTooLong    = 294
	StatusInvalidNickname    = 295
	StatusNicknameTooShort   = 296
	StatusNicknameTooLong    = 297
	StatusInvalidDeviceID    = 298
	StatusServerError        = 500
)

// Стандартные сообщения об ошибках
var errorMessages = map[int]string{
	StatusInvalidRequest:     "Invalid request. Please update to the latest version.",
	StatusNoPermission:       "You don't have permission to perform this action.",
	StatusInvalidCredentials: "Account or password is incorrect! If you forget your password, please reset it.",
	StatusAccountNotExist:    "Account does not exist.",
	StatusEmailAlreadyTaken:  "This email address is already registered.",
	StatusInvalidPassword:    "Invalid password.",
	StatusInvalidVerifyCode:  "Invalid verification code.",
	StatusInvalidAPIKey:      "Invalid or missing API key.",
	StatusMissingAPIKey:      "Invalid request. Please update to the latest version.",
	StatusDPoPLoginRequired:  "Invalid request. Please update to the latest version.",
	StatusDPoPRefreshRequired: "Invalid request. Please update to the latest version.",
	StatusDPoPBindingMissing: "Invalid request. Please update to the latest version.",
	StatusUserBanned:         "You are banned and cannot perform this action.",
	StatusBlockedByUser:      "This user has blocked you.",
	StatusReadMode:           "You are in read mode. You cannot perform this action until read mode is lifted.",
	StatusTooManyRequests:    "Too many requests. Please try again later.",
	StatusEmailRateLimited:   "Too many verification emails. Please try again in 10 minutes.",
	StatusWeakPassword:       "Password is too weak.",
	StatusPasswordTooShort:   "Password must be at least 6 characters.",
	StatusPasswordTooLong:    "Password must be no more than 128 characters.",
	StatusInvalidNickname:    "Invalid nickname.",
	StatusNicknameTooShort:   "Nickname must be at least 3 characters.",
	StatusNicknameTooLong:    "Nickname must be no more than 32 characters.",
	StatusInvalidDeviceID:    "Invalid device ID.",
	StatusServerError:        "Server error. Please try again later.",
}

// NewError создаёт ошибку с кодом Amino
func NewError(code int) ErrorResponse {
	msg, ok := errorMessages[code]
	if !ok {
		msg = "Unknown error"
	}
	return ErrorResponse{
		StatusCode: code,
		Message:    msg,
	}
}

// NewErrorWithMessage создаёт ошибку с кастомным сообщением
func NewErrorWithMessage(code int, message string) ErrorResponse {
	return ErrorResponse{
		StatusCode: code,
		Message:    message,
	}
}

// InvalidRequest - самая частая ошибка
func InvalidRequest() ErrorResponse {
	return NewError(StatusInvalidRequest)
}

// InvalidCredentials - неверный логин/пароль
func InvalidCredentials() ErrorResponse {
	return NewError(StatusInvalidCredentials)
}

// BlockedByUser - пользователь вас заблокировал
func BlockedByUser() ErrorResponse {
	return NewError(StatusBlockedByUser)
}
