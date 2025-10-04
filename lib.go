package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	timeFormat   = time.StampMilli
	LevelTrace   = slog.Level(-8)
	LevelFatal   = slog.Level(12)
	bold         = "\033[1m"
	reset        = "\033[0m"
	black        = "\033[30m"
	red          = "\033[31m"
	green        = "\033[32m"
	yellow       = "\033[33m"
	blue         = "\033[34m"
	magenta      = "\033[35m"
	cyan         = "\033[36m"
	lightGray    = "\033[37m"
	darkGray     = "\033[90m"
	lightRed     = "\033[91m"
	lightGreen   = "\033[92m"
	lightYellow  = "\033[93m"
	lightBlue    = "\033[94m"
	lightMagenta = "\033[95m"
	lightCyan    = "\033[96m"
	white        = "\033[97m"
)

type Handler struct {
	h slog.Handler
	b *bytes.Buffer
	m *sync.Mutex
}

func init() {
	//bi, _ := debug.ReadBuildInfo()

	logOptions := slog.HandlerOptions{
		Level:     LevelTrace,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				valStrParts := strings.Split(a.Value.String(), " ")
				if len(valStrParts[1]) > 40 {
					codePath := ".." + valStrParts[1][len(valStrParts[1])-20:]
					lineNumber := valStrParts[2][0 : len(valStrParts[2])-1]
					a.Value = slog.StringValue(codePath + ":" + lineNumber)
				}
			}
			return a
		},
	}
	logger := slog.New(NewHandler(&logOptions))
	slog.SetDefault(logger)

	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		//set default to trace
		lvl = "TRACE"

	}

	switch lvl {
	case "TRACE":
		slog.SetLogLoggerLevel(LevelTrace)
	case "FATAL":
		slog.SetLogLoggerLevel(LevelFatal)
	case "ERROR":
		slog.SetLogLoggerLevel(slog.LevelError)
	case "WARN":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "INFO":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "DEBUG":
		slog.SetLogLoggerLevel(slog.LevelDebug)

	}

	slog.Info("Loger setup completed")
	slog.Info("Log Level set to " + lvl)
}

func NewHandler(opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	b := &bytes.Buffer{}
	return &Handler{
		b: b,
		h: slog.NewJSONHandler(b, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: suppressDefaults(opts.ReplaceAttr),
		}),
		m: &sync.Mutex{},
	}
}

func suppressDefaults(
	next func([]string, slog.Attr) slog.Attr,
) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey ||
			a.Key == slog.LevelKey ||
			a.Key == slog.MessageKey {
			return slog.Attr{}
		}
		if next == nil {
			return a
		}
		return next(groups, a)
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs), b: h.b, m: h.m}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name), b: h.b, m: h.m}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {

	attrs, err := h.computeAttrs(ctx, r)
	if err != nil {
		return err
	}

	// get the source attribute and format it as {"source": "file:line"} and marshal to sourceBytes
	var sourceBytes []byte
	var jsonString string
	if src, ok := attrs["source"]; ok {
		jsonString = fmt.Sprintf(`{"src": "%s"}`, src)
		sourceBytes = []byte(jsonString)
	} else {
		jsonString = `{"src": "unknown"}`
		sourceBytes = []byte(jsonString)
	}
	delete(attrs, "source")

	messageBytes, err := json.Marshal(attrs)
	if err != nil {
		messageBytes = []byte("")
	}

	level := fmt.Sprintf("%6s", r.Level.String()+":")
	time := r.Time.Format(timeFormat)

	message := fmt.Sprintf("%s %s %s %s %s", time, string(sourceBytes), level, r.Message, messageBytes)

	switch r.Level {
	case slog.LevelDebug:
		message = colorize(lightBlue, message)
	case slog.LevelInfo:
		message = colorize(cyan, message)
	case slog.LevelWarn:
		message = colorize(yellow, message)
	case slog.LevelError:
		message = colorize(red, message)
	}

	fmt.Println(message)

	return nil
}

func (h *Handler) computeAttrs(
	ctx context.Context,
	r slog.Record,
) (map[string]any, error) {
	h.m.Lock()
	defer func() {
		h.b.Reset()
		h.m.Unlock()
	}()
	if err := h.h.Handle(ctx, r); err != nil {
		return nil, fmt.Errorf("error when calling inner handler's Handle: %w", err)
	}

	var attrs map[string]any
	err := json.Unmarshal(h.b.Bytes(), &attrs)
	if err != nil {
		return nil, fmt.Errorf("error when unmarshaling inner handler's Handle result: %w", err)
	}
	return attrs, nil
}

func colorize(colorCode string, v string) string {
	return fmt.Sprintf("%s%s%s%s", bold, colorCode, v, reset)
}
