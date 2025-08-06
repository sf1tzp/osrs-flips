package logging

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus with structured logging for the bot
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new structured logger configured for container environments
func NewLogger(level, format string) *Logger {
	logger := logrus.New()

	// Set log level
	switch strings.ToLower(level) {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logger.SetLevel(logrus.FatalLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set output format
	switch strings.ToLower(format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		// Default to JSON for containers
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	}

	// Set output (stdout for containers)
	logger.SetOutput(os.Stdout)

	return &Logger{Logger: logger}
}

// WithComponent adds a component field to all log entries
func (l *Logger) WithComponent(component string) *logrus.Entry {
	return l.WithField("component", component)
}

// WithJob adds job context to log entries
func (l *Logger) WithJob(jobName string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"component": "job_executor",
		"job_name":  jobName,
	})
}

// WithDiscord adds Discord context to log entries
func (l *Logger) WithDiscord() *logrus.Entry {
	return l.WithField("component", "discord_bot")
}

// WithLLM adds LLM context to log entries
func (l *Logger) WithLLM() *logrus.Entry {
	return l.WithField("component", "llm_client")
}

// WithOSRS adds OSRS API context to log entries
func (l *Logger) WithOSRS() *logrus.Entry {
	return l.WithField("component", "osrs_api")
}

// WithJobExecution adds job execution context
func (l *Logger) WithJobExecution(jobName string, executionID string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"component":    "job_executor",
		"job_name":     jobName,
		"execution_id": executionID,
	})
}

// WithRequestID adds request tracking
func (l *Logger) WithRequestID(requestID string) *logrus.Entry {
	return l.WithField("request_id", requestID)
}

// WithUserID adds user context for Discord interactions
func (l *Logger) WithUserID(userID string) *logrus.Entry {
	return l.WithField("user_id", userID)
}

// WithError adds error context with stack trace
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.WithField("error", err.Error())
}

// WithMetrics adds performance metrics
func (l *Logger) WithMetrics(metrics map[string]interface{}) *logrus.Entry {
	fields := logrus.Fields{"component": "metrics"}
	for k, v := range metrics {
		fields[k] = v
	}
	return l.WithFields(fields)
}

// Close provides a no-op close method for compatibility
func (l *Logger) Close() error {
	return nil
}

// SetOutput sets the logger output destination
func (l *Logger) SetOutput(output io.Writer) {
	l.Logger.SetOutput(output)
}

// JobStart logs the start of a job execution
func (l *Logger) JobStart(jobName string, executionID string) {
	l.WithJobExecution(jobName, executionID).Info("Job execution started")
}

// JobComplete logs successful job completion
func (l *Logger) JobComplete(jobName string, executionID string, duration float64, itemsProcessed int) {
	l.WithJobExecution(jobName, executionID).WithFields(logrus.Fields{
		"duration_seconds": duration,
		"items_processed":  itemsProcessed,
	}).Info("Job execution completed successfully")
}

// JobError logs job execution errors
func (l *Logger) JobError(jobName string, executionID string, err error, duration float64) {
	l.WithJobExecution(jobName, executionID).WithFields(logrus.Fields{
		"duration_seconds": duration,
		"error":            err.Error(),
	}).Error("Job execution failed")
}

// APICall logs API call attempts
func (l *Logger) APICall(component, endpoint string, method string) {
	l.WithField("component", component).WithFields(logrus.Fields{
		"endpoint": endpoint,
		"method":   method,
	}).Debug("API call initiated")
}

// APISuccess logs successful API responses
func (l *Logger) APISuccess(component, endpoint string, duration float64, statusCode int) {
	l.WithField("component", component).WithFields(logrus.Fields{
		"endpoint":    endpoint,
		"duration_ms": duration * 1000,
		"status_code": statusCode,
	}).Debug("API call successful")
}

// APIError logs API call failures
func (l *Logger) APIError(component, endpoint string, err error, duration float64, statusCode int) {
	l.WithField("component", component).WithFields(logrus.Fields{
		"endpoint":    endpoint,
		"duration_ms": duration * 1000,
		"status_code": statusCode,
		"error":       err.Error(),
	}).Error("API call failed")
}

// DiscordMessage logs Discord message events
func (l *Logger) DiscordMessage(channelID, messageID string, length int) {
	l.WithDiscord().WithFields(logrus.Fields{
		"channel_id":     channelID,
		"message_id":     messageID,
		"message_length": length,
	}).Info("Discord message sent")
}

// DiscordError logs Discord API errors
func (l *Logger) DiscordError(action string, err error) {
	l.WithDiscord().WithFields(logrus.Fields{
		"action": action,
		"error":  err.Error(),
	}).Error("Discord operation failed")
}
