package billadjustment

import (
	"fmt"
	"time"

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
	"hcm/pkg/rest"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/encode"
	"hcm/pkg/tools/slice"
)

var (
	excelHeader = []string{"更新时间", "调账ID", "业务", "二级账号名称", "调账类型",
		"操作人", "金额", "币种", "调账状态"}
)

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

	result, err := b.fetchBillAdjustmentItem(cts, req)
	if err != nil {
		return nil, err
	}

	bizIDs := make([]int64, 0, len(result))
	mainAccountIDs := make([]string, 0, len(result))
	for _, detail := range result {
		bizIDs = append(bizIDs, detail.BkBizID)
		mainAccountIDs = append(mainAccountIDs, detail.MainAccountID)
	}

	// fetch main account
	mainAccountMap, err := b.listMainAccount(cts.Kit, mainAccountIDs)
	// fetch biz
	bizMap, err := b.listBiz(cts.Kit, bizIDs)

	data := make([][]string, 0, len(result)+1)
	data = append(data, excelHeader)
	data = append(data, toRawData(result, mainAccountMap, bizMap)...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s/bill_adjustment_item__%s.csv", constant.BillExportFolderPrefix,
		time.Now().Format("20060102150405"))
	base64Str, err := encode.ReaderToBase64Str(buf)
	if err != nil {
		return nil, err
	}
	if err = b.client.DataService().Global.Cos.Upload(cts.Kit,
		&cos.UploadFileReq{
			Filename:   filename,
			FileBase64: base64Str,
		}); err != nil {
		return nil, err
	}
	url, err := b.client.DataService().Global.Cos.GenerateTemporalUrl(cts.Kit, "download",
		&cos.GenerateTemporalUrlReq{
			Filename:   filename,
			TTLSeconds: 3600,
		})
	if err != nil {
		return nil, err
	}

	return bill.BillExportResult{DownloadURL: url.URL}, nil
}

func (b *billAdjustmentSvc) fetchBillAdjustmentItem(cts *rest.Contexts, req *bill.AdjustmentItemExportReq) (
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
	details, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(cts.Kit,
		&dsbillapi.BillAdjustmentItemListReq{
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

	result := make([]*billcore.AdjustmentItem, 0, len(details.Details))
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(cts.Kit,
			&dsbillapi.BillAdjustmentItemListReq{
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

func toRawData(details []*billcore.AdjustmentItem, accountMap map[string]*accountset.BaseMainAccount,
	bizMap map[int64]string) [][]string {

	data := make([][]string, 0, len(details))
	for _, detail := range details {
		bizName := bizMap[detail.BkBizID]
		var mainAccountID string
		if mainAccount, ok := accountMap[detail.MainAccountID]; ok {
			mainAccountID = mainAccount.CloudID
		}

		tmp := []string{
			detail.UpdatedAt,
			detail.ID,
			bizName,
			mainAccountID,
			enumor.BillAdjustmentTypeNameMap[detail.Type],
			detail.Operator,
			detail.Cost.String(),
			detail.Currency,
			enumor.BillAdjustmentStateNameMap[detail.State],
		}

		data = append(data, tmp)
	}
	return data
}

func (b *billAdjustmentSvc) listBiz(kt *kit.Kit, ids []int64) (map[int64]string, error) {
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
	expression, err := tools.And(
		tools.RuleIn("id", ids),
	)
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
