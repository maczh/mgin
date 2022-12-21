package utils

import (
	"fmt"
	"github.com/go-errors/errors"
)

type safeGo struct {
	argsF       func(args ...any)           // 动态参数函数
	goBeforeF   func() map[string]any       // 协程前的处理函数
	callBeforeF func(params map[string]any) // 调用前的处理函数
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
	preRoutineId := receiver.goBeforeF()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				goErr := errors.Wrap(err, 2)
				reset := string([]byte{27, 91, 48, 109})
				fmt.Printf("[SafeGo] panic recovered:\n\n%s%s\n\n%s",
					goErr.Error(), goErr.Stack(), reset)
			}
		}()
		receiver.callBeforeF(preRoutineId)
		receiver.argsF(args...)
	}()
}
