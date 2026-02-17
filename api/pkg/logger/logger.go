package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	UserID    *string                `json:"user_id,omitempty"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

type Logger struct {
	output io.Writer
}

var globalLogger *Logger

func New(output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{output: output}
}

func Init() {
	globalLogger = New(os.Stdout)
}

func (l *Logger) log(level LogLevel, action string, userID *string, details map[string]interface{}, err error) {
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		UserID:    userID,
		Action:    action,
		Details:   details,
		RequestID: getRequestID(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	if l.output == os.Stdout {
		var colorCode string
		switch level {
		case LevelError:
			colorCode = "\033[31m"
		case LevelWarn:
			colorCode = "\033[33m"
		default:
			colorCode = "\033[36m"
		}
		reset := "\033[0m"

		data, _ := json.Marshal(entry)
		fmt.Fprintf(l.output, "%s%s%s\n", colorCode, string(data), reset)
	} else {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(l.output, "%s\n", string(data))
	}
}

func Info(action string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelInfo, action, nil, details, nil)
	}
}

func InfoWithUser(userID string, action string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelInfo, action, &userID, details, nil)
	}
}

func Warn(action string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelWarn, action, nil, details, nil)
	}
}

func WarnWithUser(userID string, action string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelWarn, action, &userID, details, nil)
	}
}

func Error(action string, err error, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelError, action, nil, details, err)
	}
}

func ErrorWithUser(userID string, action string, err error, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.log(LevelError, action, &userID, details, err)
	}
}

func GetUserIDFromContext(c *fiber.Ctx) *string {
	if userID := c.Locals("userID"); userID != nil {
		if id, ok := userID.(string); ok {
			return &id
		}
	}
	return nil
}

func getRequestID() string {
	if pc, file, line, ok := runtime.Caller(3); ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			return fmt.Sprintf("%s:%d", file, line)
		}
	}
	return ""
}

var sensitiveFields = []string{"password", "oldPassword", "newPassword", "secret", "token", "apiKey", "apiKeySecret"}

func redactSensitiveFields(jsonMap map[string]interface{}) {
	for _, field := range sensitiveFields {
		if _, exists := jsonMap[field]; exists {
			jsonMap[field] = "[REDACTED]"
		}
	}
}

func GetRequestBodySummary(c *fiber.Ctx) string {
	body := c.Body()
	if len(body) == 0 {
		return "empty"
	}

	if len(body) > 1024 {
		return fmt.Sprintf("large (%d bytes)", len(body))
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(body, &jsonMap); err == nil {
		redactSensitiveFields(jsonMap)
		if jsonBytes, err := json.Marshal(jsonMap); err == nil {
			if len(jsonBytes) > 200 {
				return string(jsonBytes[:200]) + "..."
			}
			return string(jsonBytes)
		}
	}

	return fmt.Sprintf("binary (%d bytes)", len(body))
}

func GetResponseSizeSummary(c *fiber.Ctx) string {
	response := c.Response()
	if response == nil {
		return "unknown"
	}

	body := response.Body()
	if len(body) == 0 {
		return "empty"
	}

	if len(body) > 1024 {
		return fmt.Sprintf("large (%d bytes)", len(body))
	}

	return fmt.Sprintf("small (%d bytes)", len(body))
}

func GenerateRequestID() string {
	return uuid.New().String()
}
