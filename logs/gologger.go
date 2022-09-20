package logs

import (
	"github.com/maczh/mgin/config"
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
	logFileName := config.Config.GetConfigString("go.logger.file")
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

func (log GoLogger) Log(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "LOG", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Message(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "MSG", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Info(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "INF", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Warn(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "WRN", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Debug(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "DBG", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Error(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "ERR", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) Fatal(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "CRT", Message: message, LoggerInit: logger})
	}
}

func (log GoLogger) ReplaceMessage(message string) {
	for _, logger := range log.Loggers {
		logPrinter(LogInstance{LogType: "RSS", Message: message, LoggerInit: logger})
	}
}
