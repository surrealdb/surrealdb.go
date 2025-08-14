package testenv

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// TestLogHandler is a slog.Handler that prints message index (starting from 0)
// level, and message content, without the timestamp.
// This allows test log output to be deterministic.
type TestLogHandler struct {
	index               int
	attrs               []slog.Attr
	groups              []string // current group path
	showFrames          bool
	ignoreErrorPrefixes []string // prefixes of error messages to ignore
	ignoreDebug         bool     // whether to ignore DEBUG level messages
}

func NewTestLogHandler() *TestLogHandler {
	return &TestLogHandler{
		showFrames: false, // Omit frames by default
	}
}

func NewTestLogHandlerWithFrames() *TestLogHandler {
	return &TestLogHandler{
		showFrames: true,
	}
}

// NewTestLogHandlerWithOptions creates a TestLogHandler with custom options
func NewTestLogHandlerWithOptions(opts ...TestLogHandlerOption) *TestLogHandler {
	h := &TestLogHandler{
		showFrames: false,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// TestLogHandlerOption is a function that configures a TestLogHandler
type TestLogHandlerOption func(*TestLogHandler)

// WithFrames enables frame information in log output
func WithFrames() TestLogHandlerOption {
	return func(h *TestLogHandler) {
		h.showFrames = true
	}
}

// WithIgnoreErrorPrefixes sets prefixes for error messages that should be ignored
func WithIgnoreErrorPrefixes(prefixes ...string) TestLogHandlerOption {
	return func(h *TestLogHandler) {
		h.ignoreErrorPrefixes = append(h.ignoreErrorPrefixes, prefixes...)
	}
}

// WithIgnoreDebug configures the handler to ignore DEBUG level messages
func WithIgnoreDebug() TestLogHandlerOption {
	return func(h *TestLogHandler) {
		h.ignoreDebug = true
	}
}

//nolint:gocritic
func (h *TestLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Check if we should ignore DEBUG level messages
	if r.Level == slog.LevelDebug && h.ignoreDebug {
		return nil
	}

	// Check if this is an error message that should be ignored
	if r.Level == slog.LevelError && len(h.ignoreErrorPrefixes) > 0 {
		for _, prefix := range h.ignoreErrorPrefixes {
			if strings.HasPrefix(r.Message, prefix) {
				// Skip logging this error message
				return nil
			}
		}
	}

	var frameInfo string
	if h.showFrames {
		pcs := make([]uintptr, 10)
		runtime.Callers(0, pcs)
		n := 6
		n = min(n, len(pcs))
		pcs = pcs[n:]
		frames := runtime.CallersFrames(pcs)
		var sb strings.Builder
		for {
			frame, more := frames.Next()
			sb.WriteString(fmt.Sprintf("%s:%d ", frame.File, frame.Line))
			if !more {
				break
			}
		}
		frameInfo = sb.String() + " "
	}

	attrs := h.attrsToString(&r)
	if attrs != "" {
		fmt.Printf("[%d] %s%s: %s %s\n", h.index, frameInfo, r.Level, r.Message, attrs)
	} else {
		fmt.Printf("[%d] %s%s: %s\n", h.index, frameInfo, r.Level, r.Message)
	}
	h.index++
	return nil
}

func (h *TestLogHandler) attrsToString(r *slog.Record) string {
	var sb strings.Builder

	// Add pre-existing attributes (already formatted with their prefixes)
	for i, attr := range h.attrs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(h.formatAttrWithPrefix(attr, ""))
	}

	// Add record attributes with current group prefix
	r.Attrs(func(a slog.Attr) bool {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		prefix := ""
		if len(h.groups) > 0 {
			prefix = strings.Join(h.groups, ".") + "."
		}
		sb.WriteString(h.formatAttrWithPrefix(a, prefix))
		return true
	})
	return sb.String()
}

func (h *TestLogHandler) formatAttrWithPrefix(a slog.Attr, prefix string) string {
	// Handle Group attributes specially
	if a.Value.Kind() == slog.KindGroup {
		// Format group attributes with the group key as additional prefix
		groupPrefix := prefix + a.Key + "."
		var parts []string
		for _, ga := range a.Value.Group() {
			parts = append(parts, h.formatAttrWithPrefix(ga, groupPrefix))
		}
		return strings.Join(parts, ", ")
	}

	// Regular attribute - apply prefix if present
	if prefix != "" {
		return fmt.Sprintf("%s%s=%v", prefix, a.Key, a.Value)
	}
	return fmt.Sprintf("%s=%v", a.Key, a.Value)
}

func (h *TestLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *TestLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Store attributes with current group context
	// They will be formatted with the appropriate prefix when rendered
	newAttrs := make([]slog.Attr, 0, len(attrs))

	prefix := ""
	if len(h.groups) > 0 {
		prefix = strings.Join(h.groups, ".") + "."
	}

	for _, attr := range attrs {
		// For regular attributes, store with prefixed key
		// For group attributes, store as-is (they'll be expanded during formatting)
		if attr.Value.Kind() == slog.KindGroup {
			// For group attrs, we need to prefix the group key
			if prefix != "" {
				newAttrs = append(newAttrs, slog.Group(prefix+attr.Key, attrGroupToArgs(attr.Value.Group())...))
			} else {
				newAttrs = append(newAttrs, attr)
			}
		} else {
			if prefix != "" {
				newAttrs = append(newAttrs, slog.Any(prefix+attr.Key, attr.Value))
			} else {
				newAttrs = append(newAttrs, attr)
			}
		}
	}

	return &TestLogHandler{
		index:               h.index,
		attrs:               append(h.attrs[:len(h.attrs):len(h.attrs)], newAttrs...),
		groups:              h.groups,
		showFrames:          h.showFrames,
		ignoreErrorPrefixes: h.ignoreErrorPrefixes,
		ignoreDebug:         h.ignoreDebug,
	}
}

// Helper function to convert attrs back to args for slog.Group
func attrGroupToArgs(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value)
	}
	return args
}

func (h *TestLogHandler) WithGroup(name string) slog.Handler {
	// If the name is empty, return the receiver as per slog documentation
	if name == "" {
		return h
	}
	return &TestLogHandler{
		index:               h.index,
		attrs:               h.attrs,
		groups:              append(h.groups[:len(h.groups):len(h.groups)], name),
		showFrames:          h.showFrames,
		ignoreErrorPrefixes: h.ignoreErrorPrefixes,
		ignoreDebug:         h.ignoreDebug,
	}
}
