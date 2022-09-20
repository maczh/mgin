package utils

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

//ZIP压缩多个文件，带批量修改压缩包里的文件名功能
func ZipFiles(filename string, files []string, srcpath string, aliasnames []string) error {
	os.Remove(filename)
	newZipFile, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer newZipFile.Close()
	zipWriter := zip.NewWriter(newZipFile)
	defer zipWriter.Close()

	// 把files添加到zip中
	for i, file := range files {
		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = strings.Replace(file, srcpath, "/", -1)
		if aliasnames != nil && len(aliasnames) > i && strings.Contains(aliasnames[i], "|") {
			alias := strings.Split(aliasnames[i], "|")
			header.Name = strings.ReplaceAll(header.Name, alias[0], alias[1])
		}
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if _, err = io.Copy(writer, zipfile); err != nil {
			return err
		}
	}
	return nil
}

// Compress returns compressed bytes
func Compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)

	_, err := gzipWriter.Write(data)
	if err != nil {
		return nil, err
	}

	err = gzipWriter.Flush()
	if err != nil {
		return nil, err
	}

	err = gzipWriter.Close()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Decompress returns the decompressed bytes
func Decompress(data []byte) ([]byte, error) {
	byteReader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(byteReader)
	if err != nil {
		return nil, err
	}

	decompressedBytes, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	err = gzipReader.Close()
	if err != nil {
		return nil, err
	}
	return decompressedBytes, nil
}
