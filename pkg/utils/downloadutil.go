package utils

import (
	"errors"
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/v2/pkg/logs"
	"mime"
	"net/url"
	"path"
)

func DownloadFile(fileUrl, localPath string) (string, error) {
	resp, err := grequests.Get(fileUrl)
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
	parsedURL, _ := url.Parse(fileUrl)
	fileName := path.Base(fileUrl)
	if parsedURL != nil && parsedURL.Path != "" {
		fileName = path.Base(parsedURL.Path)
	}
	if disposition != "" {
		if _, params, parseErr := mime.ParseMediaType(disposition); parseErr == nil && params["filename"] != "" {
			fileName = path.Base(params["filename"])
		}
	}
	if fileName == "." || fileName == "/" || fileName == "" {
		return "", errors.New("文件名为空")
	}
	localFilePath := localPath + fileName
	err = resp.DownloadToFile(localFilePath)
	if err != nil {
		logs.Error("文件下载错误:{}", err.Error())
		return fileName, errors.New("文件下载错误:" + err.Error())
	}
	return localFilePath, nil
}
