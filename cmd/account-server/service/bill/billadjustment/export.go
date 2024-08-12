package billadjustment

import (
	"fmt"

	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	accountset "hcm/pkg/api/core/account-set"
	billcore "hcm/pkg/api/core/bill"
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
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/encode"
	"hcm/pkg/tools/slice"
)

const (
	defaultExportFilename = "bill_adjustment_item"
)

var (
	excelHeader = []string{"更新时间", "调账ID", "业务", "二级账号名称", "调账类型",
		"操作人", "金额", "币种", "调账状态"}
)

func getHeader() []string {
	return excelHeader
}

// ExportBillAdjustmentItem 查询调账明细
func (b *billAdjustmentSvc) ExportBillAdjustmentItem(cts *rest.Contexts) (any, error) {

	req := new(bill.AdjustmentItemExportReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := b.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	result, err := b.fetchBillAdjustmentItem(cts.Kit, req)
	if err != nil {
		logs.Errorf("fetch bill adjustment item failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	bizIDMap := make(map[int64]struct{})
	mainAccountIDMap := make(map[string]struct{})
	for _, detail := range result {
		bizIDMap[detail.BkBizID] = struct{}{}
		mainAccountIDMap[detail.MainAccountID] = struct{}{}
	}
	bizIDs := converter.MapKeyToSlice(bizIDMap)
	mainAccountIDs := converter.MapKeyToSlice(mainAccountIDMap)

	mainAccountMap, err := b.listMainAccount(cts.Kit, mainAccountIDs)
	if err != nil {
		logs.Errorf("list main account failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	bizMap, err := b.listBiz(cts.Kit, bizIDs)
	if err != nil {
		logs.Errorf("list biz failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, getHeader())
	table, err := toRawData(result, mainAccountMap, bizMap)
	if err != nil {
		logs.Errorf("convert to raw data error: %s, rid: %s", err, cts.Kit.Rid)
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
	if err = b.client.DataService().Global.Cos.Upload(cts.Kit, uploadFileReq); err != nil {
		logs.Errorf("update file failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	generateURLReq := &cos.GenerateTemporalUrlReq{
		Filename:   filename,
		TTLSeconds: 3600,
	}
	url, err := b.client.DataService().Global.Cos.GenerateTemporalUrl(cts.Kit, "download", generateURLReq)
	if err != nil {
		logs.Errorf("generate url failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return bill.BillExportResult{DownloadURL: url.URL}, nil
}

func (b *billAdjustmentSvc) fetchBillAdjustmentItem(kt *kit.Kit, req *bill.AdjustmentItemExportReq) (
	[]*billcore.AdjustmentItem, error) {

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
	totalCount, err := b.fetchBillAdjustmentItemCount(kt, expression)
	if err != nil {
		logs.Errorf("fetch bill adjustment item count failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	exportLimit := min(totalCount, req.ExportLimit)

	result := make([]*billcore.AdjustmentItem, 0, exportLimit)
	for offset := uint64(0); offset < exportLimit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		left := exportLimit - offset
		listReq := &dsbillapi.BillAdjustmentItemListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: min(uint(left), core.DefaultMaxPageLimit),
			},
		}
		tmpResult, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(kt, listReq)
		if err != nil {
			logs.Errorf("list bill adjustment item failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}

	return result, nil
}

func (b *billAdjustmentSvc) fetchBillAdjustmentItemCount(kt *kit.Kit,
	expression *filter.Expression) (uint64, error) {

	listReq := &dsbillapi.BillAdjustmentItemListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	}
	details, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(kt, listReq)
	if err != nil {
		return 0, err
	}
	return details.Count, nil
}

func toRawData(details []*billcore.AdjustmentItem, mainAccountMap map[string]*accountset.BaseMainAccount,
	bizMap map[int64]string) ([][]string, error) {

	data := make([][]string, 0, len(details))
	for _, detail := range details {
		bizName := bizMap[detail.BkBizID]
		mainAccount, ok := mainAccountMap[detail.MainAccountID]
		if !ok {
			return nil, fmt.Errorf("main account(%s) not found", detail.MainAccountID)
		}

		tmp := []string{
			detail.UpdatedAt,
			detail.ID,
			bizName,
			mainAccount.ID,
			enumor.BillAdjustmentTypeNameMap[detail.Type],
			detail.Operator,
			detail.Cost.String(),
			detail.Currency,
			enumor.BillAdjustmentStateNameMap[detail.State],
		}

		data = append(data, tmp)
	}
	return data, nil
}

func (b *billAdjustmentSvc) listBiz(kt *kit.Kit, ids []int64) (map[int64]string, error) {
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

func (b *billAdjustmentSvc) listMainAccount(kt *kit.Kit, ids []string) (map[string]*accountset.BaseMainAccount, error) {
	ids = slice.Unique(ids)
	expression, err := tools.And(tools.RuleIn("id", ids))
	if err != nil {
		return nil, err
	}

	details, err := b.client.DataService().Global.MainAccount.List(kt, &core.ListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		return nil, err
	}
	total := details.Count

	result := make(map[string]*accountset.BaseMainAccount, total)
	for offset := uint64(0); offset < total; offset = offset + uint64(core.DefaultMaxPageLimit) {
		tmpResult, err := b.client.DataService().Global.MainAccount.List(kt, &core.ListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: core.DefaultMaxPageLimit,
			},
		})
		if err != nil {
			return nil, err
		}
		for _, item := range tmpResult.Details {
			result[item.ID] = item
		}
	}

	return result, nil
}
