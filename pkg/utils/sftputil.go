package utils

import (
	"fmt"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"
)

func SftpClose(sftpClient *sftp.Client, sshClient *ssh.Client) {
	if sftpClient != nil {
		sftpClient.Close()
	}
	if sshClient != nil {
		sshClient.Close()
	}
}

// SftpConnect 使用用户名密码建立 sftp 连接。
//
// 安全提示（重要）：该函数当前的 HostKeyCallback 恒返回 nil，等价于
// ssh.InsecureIgnoreHostKey()，即**不校验服务端主机密钥**，存在中间人攻击（MITM）风险。
// 这里保留该行为是为了兼容使用自签/未登记主机密钥的存量业务，避免破坏性变更。
//
// 新代码请使用 SftpConnectWithHostKey 或 SftpConnectWithKnownHosts 显式校验主机密钥。
func SftpConnect(user, password, host string, port int) (*sftp.Client, *ssh.Client, error) {
	return sftpConnect(user, password, host, port, func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// 兼容存量行为：跳过主机密钥校验
		return nil
	})
}

// SftpConnectWithHostKey 使用用户名密码建立 sftp 连接，并由调用方提供主机密钥校验回调。
//
// hostKeyCallback 为 nil 时回退为 SftpConnect 的不校验行为。
// 常见的回调实现：
//   - knownhosts.New("<known_hosts 文件路径>") 基于 known_hosts 文件校验
//   - ssh.FixedHostKey(pubkey) 固定单一主机密钥
func SftpConnectWithHostKey(user, password, host string, port int, hostKeyCallback ssh.HostKeyCallback) (*sftp.Client, *ssh.Client, error) {
	if hostKeyCallback == nil {
		hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}
	}
	return sftpConnect(user, password, host, port, hostKeyCallback)
}

// SftpConnectWithKnownHosts 使用用户名密码建立 sftp 连接，并基于 known_hosts 文件校验主机密钥。
//
// knownHostsPath 为空时，默认使用当前用户家目录下的 ~/.ssh/known_hosts。
// 主机密钥不在 known_hosts 中或不匹配时，连接会被拒绝并返回错误。
func SftpConnectWithKnownHosts(user, password, host string, port int, knownHostsPath string) (*sftp.Client, *ssh.Client, error) {
	if knownHostsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			logs.Error("获取用户家目录失败，无法定位 known_hosts:{}", err.Error())
			return nil, nil, err
		}
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		logs.Error("加载known_hosts文件{}错误:{}", knownHostsPath, err.Error())
		return nil, nil, err
	}

	return sftpConnect(user, password, host, port, hostKeyCallback)
}

// sftpConnect 建立 sftp 连接的内部实现，hostKeyCallback 决定如何校验服务端主机密钥。
func sftpConnect(user, password, host string, port int, hostKeyCallback ssh.HostKeyCallback) (*sftp.Client, *ssh.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		sshClient    *ssh.Client
		sftpClient   *sftp.Client
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: hostKeyCallback,
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)

	if sshClient, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		logs.Error("sftp建立服务器{}连接错误:{}", addr, err.Error())
		return nil, nil, err
	}

	// create sftp client
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		logs.Error("建立sftp客户端错误:{}", err.Error())
		return nil, nil, err
	}

	return sftpClient, sshClient, nil
}

func SftpUploadFile(sftpClient *sftp.Client, localFilePath string, remotePath string) {
	srcFile, err := os.Open(localFilePath)
	if err != nil {
		logs.Error("本地文件{}打开错误:{}", localFilePath, err.Error())
	}
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		logs.Error("远程文件:{}{}创建错误:{}", remotePath, remoteFileName, err.Error())

	}
	defer dstFile.Close()

	ff, err := ioutil.ReadAll(srcFile)
	if err != nil {
		logs.Error("读取本地文件{}错误:{}", localFilePath, err.Error())
	}
	dstFile.Write(ff)
	logs.Debug("文件{}上传成功!", localFilePath)
}
