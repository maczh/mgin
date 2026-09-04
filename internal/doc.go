// Package internal 为 mgin 框架内部预留目录。
//
// 当前 v2.0 阶段尚未使用：所有对外可导入的契约均位于 pkg/ 下，
// 框架私有实现（插件注册表、生命周期引导、依赖检查、配置加载等）后续若需对
// 模块内其他包隐藏，可放入本目录。internal 目录下的包仅允许被
// github.com/maczh/mgin 模块内的代码导入。
package internal
