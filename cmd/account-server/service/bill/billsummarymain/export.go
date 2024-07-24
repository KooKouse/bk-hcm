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

package billsummarymain

import (
	"fmt"
	"time"

	"hcm/cmd/account-server/logics/bill/export"
	asbillapi "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	accountset "hcm/pkg/api/core/account-set"
	dsbillapi "hcm/pkg/api/data-service/bill"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/rest"
)

var (
	excelHeader = []string{"二级账号ID", "二级账号名称", "一级账号ID", "一级账号名称", "业务",
		"已确认账单人民币（元）", "已确认账单美金（美元）", "当前账单人民币（元）", "当前账单美金（美元）"}
)

// ExportMainAccountSummary export main account summary with options
func (s *service) ExportMainAccountSummary(cts *rest.Contexts) (interface{}, error) {
	req := new(asbillapi.MainAccountSummaryExportReq)
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

	result, err := s.fetchMainAccountSummary(cts, req)
	if err != nil {
		return nil, err
	}

	accountIDs := make([]string, 0, len(result))
	bizIDs := make([]int64, 0, len(result))
	for _, detail := range result {
		accountIDs = append(accountIDs, detail.MainAccountID)
		bizIDs = append(bizIDs, detail.BkBizID)
	}

	accountMap, err := s.listMainAccount(cts.Kit, accountIDs)

	// fetch biz
	bizMap, err := s.listBiz(cts.Kit, bizIDs)

	data := make([][]string, 0, len(result)+1)
	data = append(data, excelHeader)
	data = append(data, toRawData(result, accountMap, bizMap)...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("bill_summary_main_%s.csv", time.Now().Format("20060102150405"))
	err = s.client.DataService().Global.Cos.Upload(cts.Kit, filename, buf)
	if err != nil {
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

func (s *service) fetchMainAccountSummary(cts *rest.Contexts, req *asbillapi.MainAccountSummaryExportReq) (
	[]*dsbillapi.BillSummaryMainResult, error) {

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

	details, err := s.client.DataService().Global.Bill.ListBillSummaryMain(cts.Kit, &dsbillapi.BillSummaryMainListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		return nil, err
	}

	limit := details.Count
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*dsbillapi.BillSummaryMainResult, 0, len(details.Details))
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := s.client.DataService().Global.Bill.ListBillSummaryMain(cts.Kit, &dsbillapi.BillSummaryMainListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: page,
			},
		})
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func toRawData(details []*dsbillapi.BillSummaryMainResult, accountMap map[string]*accountset.BaseMainAccount,
	bizMap map[int64]string) [][]string {

	data := make([][]string, 0, len(details))
	for _, detail := range details {
		var mainAccountID, mainAccountName string
		if mainAccount, ok := accountMap[detail.MainAccountID]; ok {
			mainAccountID = mainAccount.CloudID
		}
		bizName := bizMap[detail.BkBizID]
		tmp := []string{
			mainAccountID,
			mainAccountName,
			detail.RootAccountID,
			detail.MainAccountName,
			bizName,
			detail.CurrentMonthRMBCostSynced.String(),
			detail.CurrentMonthCostSynced.String(),
			detail.CurrentMonthRMBCost.String(),
			detail.CurrentMonthCost.String(),
		}

		data = append(data, tmp)
	}
	return data
}
