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

package billsummaryroot

import (
	"fmt"
	"time"

	"hcm/cmd/account-server/logics/bill/export"
	asbillapi "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	dsbillapi "hcm/pkg/api/data-service/bill"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/rest"
	"hcm/pkg/tools/encode"

	"github.com/TencentBlueKing/gopkg/conv"
)

var (
	excelHeader = []string{"一级账号ID", "一级账号名称", "账号状态", "账单同步（人民币-元）当月", "账单同步（人民币-元）上月",
		"账单同步（美金-美元）当月", "账单同步（美金-美元）上月", "账单同步环比", "当前账单人民币（元）", "当前账单美金（美元）",
		"调账人民币（元）", "调账美金（美元）"}
)

// ExportRootAccountSummary export root account summary with options
func (s *service) ExportRootAccountSummary(cts *rest.Contexts) (interface{}, error) {
	req := new(asbillapi.RootAccountSummaryExportReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	err := s.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	result, err := s.fetchRootAccountSummary(cts, req)
	if err != nil {
		return nil, err
	}
	data := make([][]string, 0, len(result)+1)
	data = append(data, excelHeader)
	data = append(data, toRawData(result)...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s/bill_summary_root_%s.csv", constant.BillExportFolderPrefix,
		time.Now().Format("20060102150405"))
	base64Str, err := encode.ReaderToBase64Str(buf)
	if err != nil {
		return nil, err
	}
	if err = s.client.DataService().Global.Cos.Upload(cts.Kit,
		&cos.UploadFileReq{
			Filename:   filename,
			FileBase64: base64Str,
		}); err != nil {
		return nil, err
	}
	url, err := s.client.DataService().Global.Cos.GenerateTemporalUrl(cts.Kit, "download",
		&cos.GenerateTemporalUrlReq{
			Filename:   filename,
			TTLSeconds: 3600,
		})
	if err != nil {
		return nil, err
	}

	return asbillapi.BillExportResult{DownloadURL: url.URL}, nil
}

func (s *service) fetchRootAccountSummary(cts *rest.Contexts, req *asbillapi.RootAccountSummaryExportReq) (
	[]*dsbillapi.BillSummaryRootResult, error) {

	var expression = tools.ExpressionAnd(
		tools.RuleEqual("bill_year", req.BillYear),
		tools.RuleEqual("bill_month", req.BillMonth),
	)
	if req.Filter != nil {
		var err error
		expression, err = tools.And(req.Filter, expression)
		if err != nil {
			return nil, err
		}
	}

	details, err := s.client.DataService().Global.Bill.ListBillSummaryRoot(cts.Kit,
		&dsbillapi.BillSummaryRootListReq{
			Filter: expression,
			Page:   core.NewCountPage(),
		})
	if err != nil {
		return nil, err
	}

	limit := *details.Count
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*dsbillapi.BillSummaryRootResult, 0, *details.Count)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := s.client.DataService().Global.Bill.ListBillSummaryRoot(cts.Kit,
			&dsbillapi.BillSummaryRootListReq{
				Filter: expression,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: core.DefaultMaxPageLimit,
				},
			})
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func toRawData(details []*dsbillapi.BillSummaryRootResult) [][]string {
	data := make([][]string, 0, len(details))
	for _, detail := range details {
		tmp := []string{
			detail.RootAccountID,
			detail.RootAccountName,
			enumor.RootAccountBillSummaryStateMap[detail.State],
			detail.CurrentMonthRMBCostSynced.String(),
			detail.LastMonthRMBCostSynced.String(),
			detail.CurrentMonthCostSynced.String(),
			detail.LastMonthCostSynced.String(),
			conv.ToString(detail.MonthOnMonthValue),
			detail.CurrentMonthRMBCost.String(),
			detail.CurrentMonthCost.String(),
			detail.AdjustmentRMBCost.String(),
			detail.AdjustmentCost.String(),
		}

		data = append(data, tmp)
	}
	return data
}
