package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

// GenerateCSV 生成csv内容
func GenerateCSV(data [][]string) (*bytes.Buffer, error) {
	// 创建CSV文件的缓冲区
	var buffer bytes.Buffer
	csvWriter := csv.NewWriter(&buffer)

	err := csvWriter.WriteAll(data)
	if err != nil {
		return nil, fmt.Errorf("write record to csv failed, err %s", err.Error())
	}

	// 刷新缓冲区，确保所有记录都写入
	csvWriter.Flush()
	return &buffer, nil
}
