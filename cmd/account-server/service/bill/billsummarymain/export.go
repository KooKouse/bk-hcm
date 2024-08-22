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

	"hcm/cmd/account-server/logics/bill/export"
	asbillapi "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	accountset "hcm/pkg/api/core/account-set"
	dsbillapi "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/converter"
)

const (
	defaultExportFilename = "bill_summary_main.csv"
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
		logs.Errorf("fetch main account summary error: %s, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	mainAccountIDMap := make(map[string]struct{})
	bizIDMap := make(map[int64]struct{})
	rootAccountIDMap := make(map[string]struct{})
	for _, detail := range result {
		mainAccountIDMap[detail.MainAccountID] = struct{}{}
		bizIDMap[detail.BkBizID] = struct{}{}
		rootAccountIDMap[detail.RootAccountID] = struct{}{}
	}
	mainAccountIDs := converter.MapKeyToSlice(mainAccountIDMap)
	rootAccountIDs := converter.MapKeyToSlice(rootAccountIDMap)
	bizIDs := converter.MapKeyToSlice(bizIDMap)

	mainAccountMap, err := s.listMainAccount(cts.Kit, mainAccountIDs)
	if err != nil {
		logs.Errorf("list main account error: %s, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	rootAccountMap, err := s.listRootAccount(cts.Kit, rootAccountIDs)
	if err != nil {
		logs.Errorf("list root account error: %s, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	bizMap, err := s.listBiz(cts.Kit, bizIDs)
	if err != nil {
		logs.Errorf("list biz error: %s, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, export.BillSummaryMainTableHeader)
	table, err := toRawData(cts.Kit, result, mainAccountMap, rootAccountMap, bizMap)
	if err != nil {
		logs.Errorf("convert to raw data error: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	data = append(data, table...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		logs.Errorf("generate csv failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &asbillapi.FileDownloadResp{
		ContentTypeStr:        "text/csv",
		ContentDispositionStr: fmt.Sprintf(`attachment; filename="%s"`, defaultExportFilename),
		Buffer:                buf,
	}, nil
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

	countReq := &dsbillapi.BillSummaryMainListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	}
	details, err := s.client.DataService().Global.Bill.ListBillSummaryMain(cts.Kit, countReq)
	if err != nil {
		return nil, err
	}

	exportLimit := min(details.Count, req.ExportLimit)
	result := make([]*dsbillapi.BillSummaryMainResult, 0, exportLimit)
	for offset := uint64(0); offset < exportLimit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		left := exportLimit - offset
		listReq := &dsbillapi.BillSummaryMainListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: min(uint(left), core.DefaultMaxPageLimit),
			},
		}
		tmpResult, err := s.client.DataService().Global.Bill.ListBillSummaryMain(cts.Kit, listReq)
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func toRawData(kt *kit.Kit, details []*dsbillapi.BillSummaryMainResult, mainAccountMap map[string]*accountset.BaseMainAccount,
	rootAccountMap map[string]*accountset.BaseRootAccount, bizMap map[int64]string) ([][]string, error) {

	data := make([][]string, 0, len(details))
	for _, detail := range details {

		mainAccount, ok := mainAccountMap[detail.MainAccountID]
		if !ok {
			return nil, fmt.Errorf("main account(%s) not found", detail.MainAccountID)
		}
		rootAccount, ok := rootAccountMap[detail.RootAccountID]
		if !ok {
			return nil, fmt.Errorf("root account(%s) not found", detail.RootAccountID)
		}
		bizName := bizMap[detail.BkBizID]
		table := export.BillSummaryMainTable{
			MainAccountID:             mainAccount.CloudID,
			MainAccountName:           mainAccount.Name,
			RootAccountID:             rootAccount.CloudID,
			RootAccountName:           rootAccount.Name,
			BKBizName:                 bizName,
			CurrentMonthRMBCostSynced: detail.CurrentMonthRMBCostSynced.String(),
			CurrentMonthCostSynced:    detail.CurrentMonthCostSynced.String(),
			CurrentMonthRMBCost:       detail.CurrentMonthRMBCost.String(),
			CurrentMonthCost:          detail.CurrentMonthCost.String(),
		}
		fields, err := table.GetHeaderFields()
		if err != nil {
			logs.Errorf("get header fields failed: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		data = append(data, fields)
	}
	return data, nil
}
