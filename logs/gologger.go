package logs

import (
	"fmt"
	"os"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/middleware/trace"
)

const (
	CONSOLE       string = "console"
	FILE          string = "file"
	ELASTICSEARCH string = "es"
	SimpleLog     string = "simple"
	ColoredLog    string = "color"
)

type GoLogger struct {
	Loggers []Logger
}

func GetLogger(selector ...string) GoLogger {
	logFileName := config.Config.Logger.File
	if len(selector) == 0 {
		if logFileName != "" {
			selector = []string{CONSOLE, FILE}
		} else {
			selector = []string{CONSOLE}
		}
	}
	loggers := make([]Logger, 0)
	for _, sel := range selector {
		switch sel {
		case CONSOLE:
			loggers = append(loggers, Logger{CONSOLE, ColoredLog})
		case FILE:
			if logFileName != "" {
				loggers = append(loggers, Logger{FILE, logFileName})
			}
		}
	}
	return GoLogger{loggers}
}

// write sends one event to the configured destinations.
func (log GoLogger) write(level, message string) {
	if len(config.Config.Logger.Outputs) > 0 {
		levels := map[string]string{"LOG": "info", "MSG": "info", "INF": "info", "WRN": "warn", "DBG": "debug", "ERR": "error", "CRT": "error", "RSS": "info"}
		writeRecord(captureRecord(levels[level], message), config.Config.Logger.Outputs)
		return
	}
	message = fmt.Sprintf("[goroutine_id=%d] %s", trace.GetGoroutineID(), message)
	for _, destination := range log.Loggers {
		logPrinter(LogInstance{LogType: level, Message: message, LoggerInit: destination})
	}
}

func (log GoLogger) Log(message string)            { log.write("LOG", message) }
func (log GoLogger) Message(message string)        { log.write("MSG", message) }
func (log GoLogger) Info(message string)           { log.write("INF", message) }
func (log GoLogger) Warn(message string)           { log.write("WRN", message) }
func (log GoLogger) Debug(message string)          { log.write("DBG", message) }
func (log GoLogger) Error(message string)          { log.write("ERR", message) }
func (log GoLogger) Fatal(message string)          { log.write("CRT", message); os.Exit(1) }
func (log GoLogger) ReplaceMessage(message string) { log.write("RSS", message) }
