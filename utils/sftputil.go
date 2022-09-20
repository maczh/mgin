package utils

import (
	"fmt"
	"github.com/maczh/mgin/logs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"net"
	"os"
	"path"
	"time"
)

func SftpClose(sftpClient *sftp.Client, sshClient *ssh.Client) {
	sftpClient.Close()
	sshClient.Close()
}

func SftpConnect(user, password, host string, port int) (*sftp.Client, *ssh.Client, error) {
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
		User:    user,
		Auth:    auth,
		Timeout: 30 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
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
