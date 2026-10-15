package logs

import (
	"os"
	"time"
)

func logPrinter(log LogInstance) {
	info := retrieveCallInfo()
	timer := time.Now()
	logPrint(log, info, timer)
}

func logPrint(log LogInstance, info *callerInfo, time time.Time) {
	Print(log, info.packageName, info.fileName, info.line, info.funcName, time)
	if log.LogType == "CRT" {
		os.Exit(1)
	}
}

type callerInfo struct {
	packageName string
	fileName    string
	funcName    string
	line        int
}

func retrieveCallInfo() *callerInfo {
	r := captureRecord("", "")
	return &callerInfo{packageName: r.Package, fileName: r.File, line: r.Line, funcName: r.Function}
}
