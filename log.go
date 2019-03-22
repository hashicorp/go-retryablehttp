package retryablehttp

import (
	"io"
	"log"
)

// Logger interface allows to use other loggers than
// standard log.Logger.
type Logger interface {
	Errorf(string, ...interface{})
	Debugf(string, ...interface{})
}

type stdLogger struct {
	log *log.Logger
}

// New creates a new Logger. The out variable sets the
// destination to which log data will be written.
// The prefix appears at the beginning of each generated log line.
// The flag argument defines the logging properties.
func NewStdLogger(out io.Writer, prefix string, flag int) Logger {
	return &stdLogger{log: log.New(out, prefix, flag)}
}

// Errorf logs a message at level Error on the logger.
func (l *stdLogger) Errorf(format string, v ...interface{}) {
	l.log.Printf("[ERR] "+format, v...)
}

// Debugf logs a message at level Debug on the logger.
func (l *stdLogger) Debugf(format string, v ...interface{}) {
	l.log.Printf("[DEBUG] "+format, v...)
}
