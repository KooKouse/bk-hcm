package export

import (
	"bytes"

	"hcm/pkg/logs"

	"github.com/xuri/excelize/v2"
)

const (
	defaultSheetName = "Sheet1"
)

// GenerateExcel 生成 Excel 文件
func GenerateExcel(data [][]interface{}) (*bytes.Buffer, error) {
	// 创建一个新的 Excel 文档
	f := excelize.NewFile()

	// 设置工作表的名称为 "Sheet1"
	_, err := f.NewSheet(defaultSheetName)
	if err != nil {
		return nil, err
	}

	// 写入数据到 Excel
	for row, rowData := range data {
		for col, value := range rowData {
			cell, err := excelize.CoordinatesToCellName(col+1, row+1)
			if err != nil {
				return nil, err
			}
			if err = f.SetCellValue(defaultSheetName, cell, value); err != nil {
				logs.Errorf("write value (%v) to cell[%d,%d] error: %v", value, col+1, row+1, err.Error())
				return nil, err
			}
		}
	}

	return f.WriteToBuffer()
}
