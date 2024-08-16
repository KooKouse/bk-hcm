/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"
)

// NewCsvWriter ...
func NewCsvWriter() (*bytes.Buffer, *csv.Writer) {
	var buffer bytes.Buffer
	return &buffer, csv.NewWriter(&buffer)
}

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
