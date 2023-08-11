// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils

import (
	"fmt"
	"github.com/maczh/mgin/logs"
	"os/exec"
	"runtime"
)

func AddPortsToFirewall(ports []int) {
	for _, port := range ports {
		// Linux
		if runtime.GOOS == "linux" {
			// firewalld
			firewallCmd := "firewall-cmd"
			err := exec.Command(firewallCmd, fmt.Sprintf("--add-port=%d/tcp", port), "--permanent").Run()
			if err != nil {
				logs.Error("add port to firewall failed: ", err.Error())
			}
		}
	}
}
