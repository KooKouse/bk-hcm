package encode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Base64StrToReader converts a base64 string to an io.Reader.
func Base64StrToReader(str string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(str))
}

// ReaderToBase64Str converts an io.Reader to a base64 string.
func ReaderToBase64Str(reader io.Reader) (string, error) {
	// 创建一个base64编码器，它实现了io.Writer接口
	var b64 bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &b64)

	// 创建一个管道，将reader的数据写入到encoder中
	// 这里使用io.Copy进行数据的复制操作，它会在遇到EOF时停止
	if _, err := io.Copy(encoder, reader); err != nil {
		return "", fmt.Errorf("failed to copy data to base64 encoder: %v", err)
	}

	// 必须关闭编码器以确保所有数据都被写入
	if err := encoder.Close(); err != nil {
		return "", err
	}

	// 此时b64中包含了base64编码后的数据
	return b64.String(), nil
}
