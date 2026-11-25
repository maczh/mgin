package utils

import (
	"fmt"
	"runtime/debug"

	"github.com/maczh/mgin/v2/internal/safego"
	"github.com/maczh/mgin/v2/pkg/logs"
)

type safeGo struct {
	argsF       func(args ...any)           // 动态参数函数
	goBeforeF   func() map[string]any       // 协程前的处理函数
	callBeforeF func(params map[string]any) // 调用前的处理函数
}

// GoSafe 在独立协程中执行 fn，并统一恢复 fn 内未处理的 panic。
// 项目内需要启动普通后台任务时优先使用此入口，避免裸 go func。
func GoSafe(fn func()) {
	if fn == nil {
		return
	}
	safego.Run(fn)
}

// NewSafeGo 创建一个安全的协程调用
/*
	示例:
	safeGo := NewSafeGo(func(args ...any) {

	})
	safeGo.SetGoBeforeHandler(func() map[string]any {
		return map[string]any{
			"preRoutineId": "123",
		}
	})
	safeGo.SetCallBeforeHandler(func(params map[string]any) {
		fmt.Println(params["preRoutineId"])
	})
	safeGo.Run("hello", "world")
*/
func NewSafeGo(argsF func(args ...any)) *safeGo {
	return &safeGo{
		argsF: argsF,
	}
}

// SetGoBeforeHandler 设置协程前的处理函数
func (receiver *safeGo) SetGoBeforeHandler(goBeforeF func() map[string]any) *safeGo {
	receiver.goBeforeF = goBeforeF
	return receiver
}

// SetCallBeforeHandler 设置调用前的处理函数
func (receiver *safeGo) SetCallBeforeHandler(callBeforeF func(params map[string]any)) *safeGo {
	receiver.callBeforeF = callBeforeF
	return receiver
}

// Run 运行
func (receiver *safeGo) Run(args ...any) {
	if receiver == nil || receiver.argsF == nil {
		return
	}
	var preRoutineId map[string]any
	if receiver.goBeforeF != nil {
		var ok bool
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					logs.Error("SafeGo before hook panic: {}\n{}", fmt.Sprint(recovered), string(debug.Stack()))
				}
			}()
			preRoutineId = receiver.goBeforeF()
			ok = true
		}()
		if !ok {
			return
		}
	}
	safego.Run(func() {
		if receiver.callBeforeF != nil {
			receiver.callBeforeF(preRoutineId)
		}
		receiver.argsF(args...)
	})
}
