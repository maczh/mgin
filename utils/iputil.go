package utils

import (
	"fmt"
	"github.com/maczh/mgin/logs"
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

// IsPortUse 判断端口是否被占用
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
