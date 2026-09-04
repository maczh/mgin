// Package logs 是 pkg/logs 的向后兼容别名壳。
//
// v2.0 起日志包已迁至 github.com/maczh/mgin/pkg/logs，本包仅做 re-export，
// 以便存量项目 `import "github.com/maczh/mgin/logs"` 仍可编译。该别名壳计划在
// 下一个 release 删除，新项目请直接使用 pkg/logs。
package logs

import pkglogs "github.com/maczh/mgin/pkg/logs"

// 类型别名：方法随类型自动带入（如 *Color.Set()、GoLogger.Info() 等）。
type (
	Logger      = pkglogs.Logger
	LogInstance = pkglogs.LogInstance
	GoLogger    = pkglogs.GoLogger
	Color       = pkglogs.Color
	Attribute   = pkglogs.Attribute
)

// 包级变量。
var (
	NoColor = pkglogs.NoColor
	Output  = pkglogs.Output
)

// 包级函数。
var (
	Print          = pkglogs.Print
	FilePrinter    = pkglogs.FilePrinter
	GetLogger      = pkglogs.GetLogger
	OutPrint       = pkglogs.OutPrint
	Debug          = pkglogs.Debug
	Info           = pkglogs.Info
	Warn           = pkglogs.Warn
	Error          = pkglogs.Error
	ConsolePrinter = pkglogs.ConsolePrinter
	New            = pkglogs.New
	Set            = pkglogs.Set
	Unset          = pkglogs.Unset
	Black          = pkglogs.Black
	Red            = pkglogs.Red
	Green          = pkglogs.Green
	Yellow         = pkglogs.Yellow
	Blue           = pkglogs.Blue
	Magenta        = pkglogs.Magenta
	Cyan           = pkglogs.Cyan
	White          = pkglogs.White
	BlackString    = pkglogs.BlackString
	RedString      = pkglogs.RedString
	GreenString    = pkglogs.GreenString
	YellowString   = pkglogs.YellowString
	BlueString     = pkglogs.BlueString
	MagentaString  = pkglogs.MagentaString
	CyanString     = pkglogs.CyanString
	WhiteString    = pkglogs.WhiteString
)
