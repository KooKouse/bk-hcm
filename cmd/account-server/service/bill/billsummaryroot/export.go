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

	"hcm/cmd/account-server/logics/bill/export"
	asbillapi "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	accountset "hcm/pkg/api/core/account-set"
	dsbillapi "hcm/pkg/api/data-service/bill"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/encode"
	"hcm/pkg/tools/slice"

	"github.com/TencentBlueKing/gopkg/conv"
)

const (
	defaultExportFilename = "bill_summary_root"
)

var (
	excelHeader = []string{"一级账号ID", "一级账号名称", "账号状态", "账单同步（人民币-元）当月", "账单同步（人民币-元）上月",
		"账单同步（美金-美元）当月", "账单同步（美金-美元）上月", "账单同步环比", "当前账单人民币（元）", "当前账单美金（美元）",
		"调账人民币（元）", "调账美金（美元）"}
)

func getHeader() []string {
	return excelHeader
}

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
		logs.Errorf("fetch root account summary failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	rootAccountIDMap := make(map[string]struct{})
	for _, detail := range result {
		rootAccountIDMap[detail.RootAccountID] = struct{}{}
	}
	rootAccountIDs := converter.MapKeyToSlice(rootAccountIDMap)
	rootAccountMap, err := s.listRootAccount(cts.Kit, rootAccountIDs)
	if err != nil {
		logs.Errorf("list root account error: %s, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, getHeader())
	table, err := toRawData(result, rootAccountMap)
	if err != nil {
		logs.Errorf("convert to raw data failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	data = append(data, table...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		logs.Errorf("generate csv failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	filename := export.GenerateExportCSVFilename(constant.BillExportFolderPrefix, defaultExportFilename)
	base64Str, err := encode.ReaderToBase64Str(buf)
	if err != nil {
		return nil, err
	}
	uploadFileReq := &cos.UploadFileReq{
		Filename:   filename,
		FileBase64: base64Str,
	}
	if err = s.client.DataService().Global.Cos.Upload(cts.Kit, uploadFileReq); err != nil {
		logs.Errorf("update file failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	generateURLReq := &cos.GenerateTemporalUrlReq{
		Filename:   filename,
		TTLSeconds: 3600,
	}
	url, err := s.client.DataService().Global.Cos.GenerateTemporalUrl(cts.Kit, "download", generateURLReq)
	if err != nil {
		logs.Errorf("generate url failed, err: %v, rid: %s", err, cts.Kit.Rid)
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

	countReq := &dsbillapi.BillSummaryRootListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	}
	details, err := s.client.DataService().Global.Bill.ListBillSummaryRoot(cts.Kit, countReq)
	if err != nil {
		logs.Errorf("list bill summary root failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	exportLimit := min(*details.Count, req.ExportLimit)
	result := make([]*dsbillapi.BillSummaryRootResult, 0, exportLimit)
	for offset := uint64(0); offset < exportLimit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		left := exportLimit - offset
		listReq := &dsbillapi.BillSummaryRootListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: min(uint(left), core.DefaultMaxPageLimit),
			},
		}
		tmpResult, err := s.client.DataService().Global.Bill.ListBillSummaryRoot(cts.Kit, listReq)
		if err != nil {
			logs.Errorf("list bill summary root failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func toRawData(details []*dsbillapi.BillSummaryRootResult, accountMap map[string]*accountset.BaseRootAccount) (
	[][]string, error) {
	data := make([][]string, 0, len(details))
	for _, detail := range details {
		rootAccount, ok := accountMap[detail.RootAccountID]
		if !ok {
			return nil, fmt.Errorf("root account not found, id: %s", detail.RootAccountID)
		}
		tmp := []string{
			rootAccount.CloudID,
			rootAccount.Name,
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
	return data, nil
}

func (s *service) listRootAccount(kt *kit.Kit, accountIDs []string) (map[string]*accountset.BaseRootAccount, error) {
	accountIDs = slice.Unique(accountIDs)
	if len(accountIDs) == 0 {
		return nil, nil
	}
	result := make(map[string]*accountset.BaseRootAccount, len(accountIDs))
	for _, ids := range slice.Split(accountIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListWithoutFieldReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("id", ids)),
			Page:   core.NewDefaultBasePage(),
		}
		tmpResult, err := s.client.DataService().Global.RootAccount.List(kt, listReq)
		if err != nil {
			return nil, err
		}
		for _, item := range tmpResult.Details {
			result[item.ID] = item
		}
	}
	return result, nil
}
