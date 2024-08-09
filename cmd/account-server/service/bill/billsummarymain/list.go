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

	asbillapi "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	accountset "hcm/pkg/api/core/account-set"
	dsbillapi "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/slice"
)

// ListMainAccountSummary list main account summary with options
func (s *service) ListMainAccountSummary(cts *rest.Contexts) (interface{}, error) {
	req := new(asbillapi.MainAccountSummaryListReq)
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

	summary, err := s.client.DataService().Global.Bill.ListBillSummaryMain(cts.Kit, &dsbillapi.BillSummaryMainListReq{
		Filter: expression,
		Page:   req.Page,
	})
	if err != nil {
		return nil, err
	}
	if len(summary.Details) == 0 {
		return summary, nil
	}

	ret := &asbillapi.MainAccountSummaryListResult{
		Count:   0,
		Details: make([]*asbillapi.MainAccountSummaryResult, 0, len(summary.Details)),
	}

	accountIDs := make([]string, 0, len(summary.Details))
	for _, detail := range summary.Details {
		accountIDs = append(accountIDs, detail.MainAccountID)
	}

	// fetch account
	accountMap, err := s.listMainAccount(cts.Kit, accountIDs)
	if err != nil {
		return nil, err
	}

	for _, detail := range summary.Details {
		account, ok := accountMap[detail.MainAccountID]
		if !ok {
			return nil, fmt.Errorf("[ListMainAccountSummary] summaryMain(%s): mainAccount(%s) not found",
				detail.ID, detail.MainAccountID)
		}
		tmp := &asbillapi.MainAccountSummaryResult{
			BillSummaryMainResult: *detail,
			MainAccountCloudID:    account.CloudID,
			MainAccountCloudName:  account.Name,
		}
		ret.Details = append(ret.Details, tmp)
	}

	return ret, nil
}

func (s *service) listMainAccount(kt *kit.Kit, mainAccountIDs []string) (map[string]*accountset.BaseMainAccount, error) {
	if len(mainAccountIDs) == 0 {
		return nil, nil
	}

	result := make(map[string]*accountset.BaseMainAccount, len(mainAccountIDs))
	for _, ids := range slice.Split(mainAccountIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("id", ids)),
			Page:   core.NewDefaultBasePage(),
		}
		resp, err := s.client.DataService().Global.MainAccount.List(kt, listReq)
		if err != nil {
			return nil, err
		}
		for _, detail := range resp.Details {
			result[detail.ID] = detail
		}
	}
	return result, nil
}

func (s *service) listRootAccount(kt *kit.Kit, accountIDs []string) (map[string]*accountset.BaseRootAccount, error) {
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

func (s *service) listBiz(kt *kit.Kit, ids []int64) (map[int64]string, error) {
	expression := &cmdb.QueryFilter{
		Rule: &cmdb.CombinedRule{
			Condition: "AND",
			Rules: []cmdb.Rule{
				&cmdb.AtomRule{
					Field:    "bk_biz_id",
					Operator: "in",
					Value:    slice.Unique(ids),
				},
			},
		},
	}
	params := &cmdb.SearchBizParams{
		BizPropertyFilter: expression,
		Fields:            []string{"bk_biz_id", "bk_biz_name"},
	}
	resp, err := s.esbClient.Cmdb().SearchBusiness(kt, params)
	if err != nil {
		return nil, fmt.Errorf("call cmdb search business api failed, err: %v", err)
	}

	infos := resp.Info
	data := make(map[int64]string, len(infos))
	for _, biz := range infos {
		data[biz.BizID] = biz.BizName
	}

	return data, nil
}
