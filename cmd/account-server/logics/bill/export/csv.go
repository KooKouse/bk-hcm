package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"
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

// GenerateExportCSVFilename 生成导出csv文件名, format: {prefix}/{filename}_{datetime}.csv
func GenerateExportCSVFilename(prefix, filename string) string {
	curTime := time.Now().Format("20060102150405")
	return fmt.Sprintf("%s/%s_%s.csv", prefix, filename, curTime)
}
