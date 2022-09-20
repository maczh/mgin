package logs

import (
	"time"
)

func Print(log LogInstance, packageName string, fileName string, lineNumber int, funcName string, time time.Time) {
	switch log.LoggerInit.PrinterType {
	case "console":
		ConsolePrinter(log, packageName, fileName, lineNumber, funcName, time)
	case "file":
		FilePrinter(log, packageName, fileName, lineNumber, funcName, time)
	}
}
