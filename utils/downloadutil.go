package utils

import (
	"errors"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/logs"
	"strings"
)

func DownloadFile(fileUrl, localPath string) (string, error) {
	resp, err := grequests.Get(fileUrl, &grequests.RequestOptions{})
	if err != nil {
		logs.Error("文件{}下载错误:{}", fileUrl, err.Error())
		return "", errors.New("文件下载错误:" + err.Error())
	}
	disposition := ""
	if resp.Header.Get("content-disposition") != "" {
		disposition = resp.Header.Get("content-disposition")
	}
	if resp.Header.Get("Content-Disposition") != "" {
		disposition = resp.Header.Get("Content-Disposition")
	}
	fileName := fileUrl[strings.LastIndex(fileUrl, "/")+1:]
	if disposition != "" {
		strs := strings.Split(disposition, ";")
		for _, str := range strs {
			if strings.Contains(str, "filename=") {
				ext := str[strings.LastIndex(str, "."):]
				fileName = fileName[:strings.LastIndex(fileName, ".")] + ext
			}
		}
	}
	localFilePath := localPath + fileName
	err = resp.DownloadToFile(localFilePath)
	if err != nil {
		logs.Error("文件下载错误:{}", err.Error())
		return fileName, errors.New("文件下载错误:" + err.Error())
	}
	return localFilePath, nil
}
