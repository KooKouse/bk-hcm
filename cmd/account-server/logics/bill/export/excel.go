package export

import (
	"bytes"

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

	row := 1
	// 写入数据到 Excel
	for _, rowData := range data {
		for col, value := range rowData {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return nil, err
			}
			if err = f.SetCellValue(defaultSheetName, cell, value); err != nil {
				return nil, err
			}
		}
		row++
	}

	return f.WriteToBuffer()
}
