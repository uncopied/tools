package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/vrischmann/envconfig"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const tagsName = "hashtags"

type Config struct {
	LogLevel   string `envconfig:"default=info"`
	MessageKey string `envconfig:"default=message"`
	LevelKey   string `envconfig:"default=severity"`
	TimeKey    string `envconfig:"default=timestamp"`
	AppName    string `envconfig:"default=app"`
	Host       string `envconfig:"default=localhost"`
	Version    string `envconfig:"default=0.0.0"`
	DevMode    bool   `envconfig:"default=false"`
}

var (
	// global logger instance.
	global      *zap.SugaredLogger
	globalGuard sync.RWMutex

	level      = zap.NewAtomicLevelAt(zap.InfoLevel)
	defaultCfg = Config{
		LogLevel:   "info",
		MessageKey: "message",
		LevelKey:   "severity",
		TimeKey:    "timestamp",
		AppName:    "app",
		Host:       "localhost",
		Version:    "0.0.0",
		DevMode:    false,
	}
)

func init() {
	SetLogger(New(level, &defaultCfg))
}

func InitLogger(prefix, version string) error {
	var cfg struct {
		Log *Config
	}
	if err := envconfig.InitWithPrefix(&cfg, prefix); err != nil {
		return fmt.Errorf("can't get log config; err: %w", err)
	}
	cfg.Log.Version = version

	lvl, err := zapLevelFromString(cfg.Log.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to unmurshal log level: %s; err: %v", cfg.Log.LogLevel, err)
	}
	SetLogger(New(lvl, cfg.Log))

	return nil
}

type watcher interface {
	OnConfigChange(key string, callback func(interface{}))
}

func WatchAndRebuildLogger(ctx context.Context, prefix, version string, cfg *Config, w watcher) {
	w.OnConfigChange(prefix+"_LOG_LOG_LEVEL", func(newVal interface{}) {
		newLogLevel, ok := newVal.(string)
		if !ok {
			safeErrorf("Failed to cast newVal to string, got type %T", newVal)
			return
		}
		lvl, err := zapLevelFromString(newLogLevel)
		if err != nil {
			safeErrorf("Failed to unmarshal log level: %s; err: %v", newLogLevel, err)
			return
		}

		cfg.Version = version
		SetLogger(New(lvl, cfg))
	})
}

func safeErrorf(format string, args ...interface{}) {
	if l := Logger(); l != nil {
		l.Errorf(format, args...)
	}
}

func zapLevelFromString(newLogLevel string) (zap.AtomicLevel, error) {
	lvl := zap.NewAtomicLevel()
	err := lvl.UnmarshalText([]byte(newLogLevel))
	return lvl, err
}

// New creates new *zap.SugaredLogger with standard EncoderConfig
func New(lvl zapcore.LevelEnabler, cfg *Config, options ...zap.Option) *zap.SugaredLogger {
	if lvl == nil {
		lvl = level
	}
	sink := zapcore.AddSync(os.Stdout)
	options = append(options, zap.ErrorOutput(sink))

	config := zapcore.EncoderConfig{
		TimeKey:        cfg.TimeKey,
		LevelKey:       cfg.LevelKey,
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     cfg.MessageKey,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	var encoder zapcore.Encoder
	if cfg.DevMode {
		config.EncodeLevel = zapcore.LowercaseColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(config)
	} else {
		config.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(config)
	}

	return zap.New(zapcore.NewCore(encoder, sink, lvl), options...).With(getZapFields(cfg)...).Sugar()
}

func getZapFields(config *Config) []zapcore.Field {
	var fields []zapcore.Field

	if config.Version != "" {
		fields = append(fields, zap.String("version", config.Version))
	}

	if config.AppName != "" {
		fields = append(fields, zap.String("application_name", config.AppName))
	}

	if config.Host != "" {
		fields = append(fields, zap.String("host", config.Host))
	}

	return fields
}

// Logger returns current global logger.
func Logger() *zap.SugaredLogger {
	globalGuard.RLock()
	defer globalGuard.RUnlock()
	return global
}

// SetLogger sets global used logger. This function is not thread-safe.
func SetLogger(l *zap.SugaredLogger) {
	globalGuard.Lock()
	defer globalGuard.Unlock()
	global = l
}

func Debug(ctx context.Context, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		DebugKV(ctx, fmt.Sprint(args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Debug(args...)
}

func Debugf(ctx context.Context, format string, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		DebugKV(ctx, fmt.Sprintf(format, args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Debugf(format, args...)
}

func DebugKV(ctx context.Context, message string, kvs ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		kvs = append(kvs, tagsName, prepareTags(tags))
	}
	FromContext(ctx).Debugw(message, kvs...)
}

func Info(ctx context.Context, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		InfoKV(ctx, fmt.Sprint(args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Info(args...)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		InfoKV(ctx, fmt.Sprintf(format, args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Infof(format, args...)
}

func InfoKV(ctx context.Context, message string, kvs ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		kvs = append(kvs, tagsName, prepareTags(tags))
	}
	FromContext(ctx).Infow(message, kvs...)
}

func Warn(ctx context.Context, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		WarnKV(ctx, fmt.Sprint(args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Warn(args...)
}

func Warnf(ctx context.Context, format string, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		WarnKV(ctx, fmt.Sprintf(format, args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Warnf(format, args...)
}

func WarnKV(ctx context.Context, message string, kvs ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		kvs = append(kvs, tagsName, prepareTags(tags))
	}
	FromContext(ctx).Warnw(message, kvs...)
}

func Error(ctx context.Context, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		ErrorKV(ctx, fmt.Sprint(args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Error(args...)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		ErrorKV(ctx, fmt.Sprintf(format, args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Errorf(format, args...)
}

func ErrorKV(ctx context.Context, message string, kvs ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		kvs = append(kvs, tagsName, prepareTags(tags))
	}
	FromContext(ctx).Errorw(message, kvs...)
}

func Fatal(ctx context.Context, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		FatalKV(ctx, fmt.Sprint(args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Fatal(args...)
}

func Fatalf(ctx context.Context, format string, args ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		FatalKV(ctx, fmt.Sprintf(format, args...), tagsName, prepareTags(tags))
		return
	}
	FromContext(ctx).Fatalf(format, args...)
}

func FatalKV(ctx context.Context, message string, kvs ...interface{}) {
	if tags, ok := ctx.Value(loggerContextTags).([]string); ok {
		kvs = append(kvs, tagsName, prepareTags(tags))
	}
	FromContext(ctx).Fatalw(message, kvs...)
}

func prepareTags(tags []string) string {
	b := strings.Builder{}
	for _, t := range tags {
		b.WriteRune('#')
		b.WriteString(strings.ReplaceAll(t, " ", ""))
		b.WriteRune(' ')
	}
	return b.String()
}
