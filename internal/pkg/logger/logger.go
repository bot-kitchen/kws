package logger

import (
	"os"
	"sync"

	"github.com/ak/kws/internal/infrastructure/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *Logger
	once         sync.Once
)

// Logger wraps zap.Logger with additional context methods
type Logger struct {
	*zap.Logger
	component string
}

// New creates a new Logger instance from configuration
func New(cfg config.LoggingConfig) (*Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var writeSyncer zapcore.WriteSyncer
	if cfg.Output == "stdout" || cfg.Output == "" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{Logger: zapLogger}, nil
}

// SetGlobal sets the global logger instance
func SetGlobal(l *Logger) {
	once.Do(func() {
		globalLogger = l
	})
}

// Global returns the global logger instance
func Global() *Logger {
	if globalLogger == nil {
		// Return a default logger if none set
		l, _ := New(config.LoggingConfig{
			Level:  "info",
			Format: "console",
			Output: "stdout",
		})
		return l
	}
	return globalLogger
}

// WithComponent creates a child logger with a component name
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("component", component)),
		component: component,
	}
}

// WithFields creates a child logger with additional fields
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{
		Logger:    l.Logger.With(fields...),
		component: l.component,
	}
}

// WithTenant creates a child logger with tenant context
func (l *Logger) WithTenant(tenantID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("tenant_id", tenantID)),
		component: l.component,
	}
}

// WithSite creates a child logger with site context
func (l *Logger) WithSite(siteID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("site_id", siteID)),
		component: l.component,
	}
}

// WithOrder creates a child logger with order context
func (l *Logger) WithOrder(orderID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("order_id", orderID)),
		component: l.component,
	}
}

// WithKOS creates a child logger with KOS instance context
func (l *Logger) WithKOS(kosID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(zap.String("kos_id", kosID)),
		component: l.component,
	}
}
