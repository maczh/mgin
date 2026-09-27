package utils

import (
	"fmt"
	"github.com/maczh/mgin/v2/pkg/logs"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func GetLocalIpAddress() (ip string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logs.Error("err:{}", err)
		return
	}
	for _, value := range addrs {
		if ipnet, ok := value.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.String()[:7] != "169.254" {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				return
			}
		}
	}

	ip = "127.0.0.1"
	return
}

// LocalIPs return all non-loopback IPv4 addresses
func LocalIPv4s() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
		}
	}

	return ips, nil
}

// GetIPv4ByInterface return IPv4 address from a specific interface IPv4 addresses
func GetIPv4ByInterface(name string) ([]string, error) {
	var ips []string

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
		}
	}

	return ips, nil
}

func IsIntranetIP(ip string) bool {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
		return true
	}

	if strings.HasPrefix(ip, "172.") {
		// 172.16.0.0-172.31.255.255
		arr := strings.Split(ip, ".")
		if len(arr) != 4 {
			return false
		}

		second, err := strconv.ParseInt(arr[1], 10, 64)
		if err != nil {
			return false
		}

		if second >= 16 && second <= 31 {
			return true
		}
	}

	return false
}

// IsPortUse 判断端口是否被占用。
//
// 注意：该函数的返回值与其文档注释相反。true 实际表示 netstat 未匹配到该端口（端口空闲），
// false 表示 netstat 匹配到了该端口（端口可能已被占用）。
//
// Deprecated: 命名与语义相反，且实现依赖 netstat/grep 外部命令，存在以下已知问题：
//  1. 端口数字只要出现在 netstat 输出的任意位置（PID、IP 地址、其他端口号如 8080 之于 80）
//     就会被判定为命中，存在误报；
//  2. 仅支持 linux 与 windows，在其他系统（如 darwin）上恒返回 true（即"端口空闲"）。
//
// 新代码请改用 IsPortAvailable 判断端口是否可用。
// 本函数保持原有返回值语义与实现不变，以避免破坏现有调用方。
func IsPortUse(port int) bool {
	sysType := runtime.GOOS
	var (
		output         []byte
		checkStatement string
	)
	if sysType == "linux" {
		checkStatement = fmt.Sprintf("netstat -anp | grep %d ", port)
		output, _ = exec.Command("sh", "-c", checkStatement).CombinedOutput()
	}

	if sysType == "windows" {
		checkStatement = fmt.Sprintf("netstat -ano -p tcp | findstr %d", port)
		output, _ = exec.Command("cmd", "/c", checkStatement).CombinedOutput()
	}

	if len(output) > 0 {
		return false
	}
	return true
}

// IsPortAvailable 判断本机指定 TCP 端口当前是否可用（即没有被任何进程占用）。
//
// 返回 true 表示端口空闲可用，false 表示端口已被占用或端口号非法。
// 实现方式：尝试监听该端口，成功则说明空闲（随后立即释放），失败则说明被占用。
// 相比 IsPortUse，该函数不依赖 netstat/findstr 等外部命令，跨平台可用，也不会
// 因为端口数字出现在 PID 或 IP 地址中而产生误报。
func IsPortAvailable(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = listener.Close()

	return true
}
