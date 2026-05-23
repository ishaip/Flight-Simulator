package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelError LogLevel = iota
	LevelWarning
	LevelInfo
	LevelTrace
)

type Logger struct {
	sessionID     string
	file          *os.File
	mu            sync.Mutex
	logChan       chan logEntry
	done          chan struct{}
	infoEnabled   bool
	traceEnabled  bool
	broadcaster   *LogBroadcaster
}

type logEntry struct {
	level     LogLevel
	message   string
	timestamp time.Time
}

// LogEntry represents a log message to be sent to frontend.
type LogEntry struct {
	Level   string `json:"level"`   // "error", "warn", "info", "trace"
	Message string `json:"message"`
}

// LogBroadcaster fans log entries out to any number of SSE listeners.
type LogBroadcaster struct {
	mu   sync.Mutex
	subs []chan LogEntry
}

// Subscribe registers a new listener and returns its channel.
func (b *LogBroadcaster) Subscribe() <-chan LogEntry {
	ch := make(chan LogEntry, 16)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes the channel previously returned by Subscribe.
func (b *LogBroadcaster) Unsubscribe(ch <-chan LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s == ch {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			close(s)
			return
		}
	}
}

// Publish sends log entry to every subscriber. Sends are non-blocking.
func (b *LogBroadcaster) Publish(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- entry:
		default:
		}
	}
}

func NewLogger(sessionID string) (*Logger, error) {
	// Create outputs directory if it doesn't exist
	outputDir := "outputs"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Create session-specific log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("output_%s_%s.log", sessionID, timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	logger := &Logger{
		sessionID:    sessionID,
		file:         file,
		logChan:      make(chan logEntry, 1000),
		done:         make(chan struct{}),
		infoEnabled:  false,
		traceEnabled: true,
	}

	// Start background logging goroutine
	go logger.logWorker()

	return logger, nil
}

// SetLogBroadcaster sets the broadcaster for sending logs to frontend
func (l *Logger) SetLogBroadcaster(b *LogBroadcaster) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.broadcaster = b
}

func (l *Logger) logWorker() {
	for {
		select {
		case entry := <-l.logChan:
			l.writeEntry(entry)
		case <-l.done:
			// Drain remaining entries
			for {
				select {
				case entry := <-l.logChan:
					l.writeEntry(entry)
				default:
					return
				}
			}
		}
	}
}

func (l *Logger) writeEntry(entry logEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	levelStr := ""
	broadcastLevel := ""
	switch entry.level {
	case LevelError:
		levelStr = "ERROR"
		broadcastLevel = "error"
	case LevelWarning:
		levelStr = "WARNING"
		broadcastLevel = "warn"
	case LevelInfo:
		levelStr = "INFO"
		broadcastLevel = "info"
	case LevelTrace:
		levelStr = "TRACE"
		broadcastLevel = "trace"
	}

	line := fmt.Sprintf("[%s] %s: %s\n", entry.timestamp.Format("15:04:05.000"), levelStr, entry.message)
	if l.file != nil {
		l.file.WriteString(line)
	}

	// Publish to frontend via broadcaster (errors and warnings only)
	if l.broadcaster != nil && (entry.level == LevelError || entry.level == LevelWarning) {
		l.broadcaster.Publish(LogEntry{
			Level:   broadcastLevel,
			Message: entry.message,
		})
	}
}

func (l *Logger) Error(msg string) {
	entry := logEntry{
		level:     LevelError,
		message:   msg,
		timestamp: time.Now(),
	}
	select {
	case l.logChan <- entry:
	default:
		// Channel full, drop entry
	}
}

func (l *Logger) Warning(msg string) {
	entry := logEntry{
		level:     LevelWarning,
		message:   msg,
		timestamp: time.Now(),
	}
	select {
	case l.logChan <- entry:
	default:
	}
}

func (l *Logger) Info(msg string) {
	if !l.infoEnabled {
		return
	}
	entry := logEntry{
		level:     LevelInfo,
		message:   msg,
		timestamp: time.Now(),
	}
	select {
	case l.logChan <- entry:
	default:
	}
}

func (l *Logger) Trace(msg string) {
	if !l.traceEnabled {
		return
	}
	entry := logEntry{
		level:     LevelTrace,
		message:   msg,
		timestamp: time.Now(),
	}
	select {
	case l.logChan <- entry:
	default:
	}
}

func (l *Logger) SetInfoEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoEnabled = enabled
}

func (l *Logger) SetTraceEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.traceEnabled = enabled
}

func (l *Logger) Close() error {
	close(l.done)
	time.Sleep(100 * time.Millisecond) // Give worker time to flush
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
