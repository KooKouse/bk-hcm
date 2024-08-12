/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package billitem

import (
	"bytes"
	"fmt"

	"hcm/cmd/account-server/logics/bill/export"
	accountset "hcm/pkg/api/core/account-set"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/encode"
	"hcm/pkg/tools/slice"

	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/shopspring/decimal"
)

const (
	defaultExportFilename = "bill_item"
)

var (
	commonExcelHeader = []string{"站点类型", "核算年月", "业务名称", "一级帐号名称", "二级帐号名称", "地域"}

	gcpExcelHeader = []string{"Region位置", "项目ID", "项目名称", "服务分类", "服务分类名称", "Sku名称", "外币类型",
		"用量单位", "用量", "外币成本(元)", "汇率", "人民币成本(元)"}

	azureExcelHeader = []string{"区域", "地区编码", "核算年月", "业务名称",
		"账号邮箱", "子账号名称", "服务一级类别名称", "服务二级类别名称", "服务三级类别名称", "产品名称", "资源类别",
		"计量地区", "资源地区编码", "单位", "用量", "折后税前成本（外币）", "币种", "汇率", "RMB成本（元）"}

	huaWeiExcelHeader = []string{"产品名称", "云服务区名称", "金额单位", "使用量类型", "使用量度量单位", "云服务类型编码",
		"云服务类型名称", "资源类型编码", "资源类型名称", "计费模式", "账单类型", "套餐内使用量", "使用量", "预留实例使用量", "币种",
		"汇率", "本期应付外币金额（元）", "本期应付人民币金额（元）"}

	awsExcelHeader = []string{"地区名称", "发票ID", "账单实体", "产品代号", "服务组", "产品名称", "API操作", "产品规格",
		"实例类型", "资源ID", "计费方式", "计费类型", "计费说明", "用量", "单位", "折扣前成本（外币）", "外币种类",
		"人民币成本（元）", "汇率"}
)

// ExportBillItems 导出账单明细
func (b *billItemSvc) ExportBillItems(cts *rest.Contexts) (any, error) {
	vendor := enumor.Vendor(cts.PathParameter("vendor").String())
	if len(vendor) == 0 {
		return nil, errf.New(errf.InvalidParameter, "vendor is required")
	}

	req := new(bill.ExportBillItemReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := b.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	rate, err := b.getExchangeRate(cts.Kit, req.BillYear, req.BillMonth)
	if err != nil {
		logs.Errorf("fail get exchange rate for exporting bill items, err: %v, year: %d, month: %d, rid: %s",
			err, req.BillYear, req.BillMonth, cts.Kit.Rid)
		return nil, err
	}

	switch vendor {
	case enumor.HuaWei:
		return b.exportHuaweiBillItems(cts.Kit, req, rate)
	case enumor.Gcp:
		return b.exportGcpBillItems(cts.Kit, req, rate)
	case enumor.Aws:
		return b.exportAwsBillItems(cts.Kit, req, rate)
	default:
		return nil, fmt.Errorf("unsupport %s vendor", vendor)
	}
}

func (b *billItemSvc) getExchangeRate(kt *kit.Kit, year, month int) (*decimal.Decimal, error) {
	// 获取汇率
	result, err := b.client.DataService().Global.Bill.ListExchangeRate(kt, &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("from_currency", enumor.CurrencyUSD),
			tools.RuleEqual("to_currency", enumor.CurrencyRMB),
			tools.RuleEqual("year", year),
			tools.RuleEqual("month", month),
		),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get exchange rate from %s to %s in %d-%d failed, err %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, year, month, err.Error())
	}
	if len(result.Details) == 0 {
		return nil, fmt.Errorf("get no exchange rate from %s to %s in %d-%d, rid %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, year, month, kt.Rid)
	}
	if result.Details[0].ExchangeRate == nil {
		return nil, fmt.Errorf("get exchange rate is nil, from %s to %s in %d-%d, rid %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, year, month, kt.Rid)
	}
	return result.Details[0].ExchangeRate, nil
}

func (b *billItemSvc) uploadFileAndReturnUrl(kt *kit.Kit, buf *bytes.Buffer) (string, error) {
	filename := export.GenerateExportCSVFilename(constant.BillExportFolderPrefix, defaultExportFilename)
	base64Str, err := encode.ReaderToBase64Str(buf)
	if err != nil {
		return "", err
	}
	uploadReq := &cos.UploadFileReq{
		Filename:   filename,
		FileBase64: base64Str,
	}
	if err = b.client.DataService().Global.Cos.Upload(kt, uploadReq); err != nil {
		return "", err
	}

	generateReq := &cos.GenerateTemporalUrlReq{
		Filename:   filename,
		TTLSeconds: 3600,
	}
	result, err := b.client.DataService().Global.Cos.GenerateTemporalUrl(kt, "download", generateReq)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (b *billItemSvc) listMainAccountByIDs(kt *kit.Kit, mainAccountIDs []string) (map[string]*accountset.BaseMainAccount, error) {
	if len(mainAccountIDs) == 0 {
		return nil, nil
	}

	result := make(map[string]*accountset.BaseMainAccount, len(mainAccountIDs))
	for _, ids := range slice.Split(mainAccountIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("id", ids)),
			Page:   core.NewDefaultBasePage(),
		}
		resp, err := b.client.DataService().Global.MainAccount.List(kt, listReq)
		if err != nil {
			return nil, err
		}
		for _, detail := range resp.Details {
			result[detail.ID] = detail
		}
	}
	return result, nil
}

func (b *billItemSvc) listRootAccount(kt *kit.Kit,
	rootAccountIDs []string) (map[string]*accountset.BaseRootAccount, error) {

	rootAccountIDs = slice.Unique(rootAccountIDs)
	if len(rootAccountIDs) == 0 {
		return nil, nil
	}

	result := make(map[string]*accountset.BaseRootAccount, len(rootAccountIDs))
	for _, ids := range slice.Split(rootAccountIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListWithoutFieldReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("id", ids)),
			Page:   core.NewDefaultBasePage(),
		}
		tmpResult, err := b.client.DataService().Global.RootAccount.List(kt, listReq)
		if err != nil {
			return nil, err
		}
		for _, item := range tmpResult.Details {
			result[item.ID] = item
		}
	}
	return result, nil
}

func (b *billItemSvc) listBiz(kt *kit.Kit, ids []int64) (map[int64]string, error) {
	ids = slice.Unique(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	rules := []cmdb.Rule{
		&cmdb.AtomRule{
			Field:    "bk_biz_id",
			Operator: "in",
			Value:    ids,
		},
	}
	expression := &cmdb.QueryFilter{Rule: &cmdb.CombinedRule{Condition: "AND", Rules: rules}}
	params := &cmdb.SearchBizParams{
		BizPropertyFilter: expression,
		Fields:            []string{"bk_biz_id", "bk_biz_name"},
	}
	resp, err := b.esbClient.Cmdb().SearchBusiness(kt, params)
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

// prepareRelateData 准备关联数据
func (b *billItemSvc) fetchAccountBizInfo(kt *kit.Kit, rootAccountIDs, mainAccountIDs []string, bkBizIDs []int64) (
	rootAccountMap map[string]*accountset.BaseRootAccount, mainAccountMap map[string]*accountset.BaseMainAccount,
	bizNameMap map[int64]string, err error) {

	bizNameMap, err = b.listBiz(kt, bkBizIDs)
	if err != nil {
		logs.Errorf("fail to list biz, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, nil, err
	}
	mainAccountMap, err = b.listMainAccountByIDs(kt, mainAccountIDs)
	if err != nil {
		logs.Errorf("fail to list main account, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, nil, err
	}
	rootAccountMap, err = b.listRootAccount(kt, rootAccountIDs)
	if err != nil {
		logs.Errorf("fail to list root account, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, nil, err
	}
	return rootAccountMap, mainAccountMap, bizNameMap, nil
}
