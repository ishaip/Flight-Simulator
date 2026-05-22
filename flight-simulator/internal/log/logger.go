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
	sessionID  string
	file       *os.File
	mu         sync.Mutex
	logChan    chan logEntry
	done       chan struct{}
	infoEnabled bool
	traceEnabled bool
}

type logEntry struct {
	level     LogLevel
	message   string
	timestamp time.Time
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
		sessionID: sessionID,
		file:      file,
		logChan:   make(chan logEntry, 1000),
		done:      make(chan struct{}),
		infoEnabled: false,
		traceEnabled: false,
	}

	// Start background logging goroutine
	go logger.logWorker()

	return logger, nil
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
	switch entry.level {
	case LevelError:
		levelStr = "ERROR"
	case LevelWarning:
		levelStr = "WARNING"
	case LevelInfo:
		levelStr = "INFO"
	case LevelTrace:
		levelStr = "TRACE"
	}

	line := fmt.Sprintf("[%s] %s: %s\n", entry.timestamp.Format("15:04:05.000"), levelStr, entry.message)
	if l.file != nil {
		l.file.WriteString(line)
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
