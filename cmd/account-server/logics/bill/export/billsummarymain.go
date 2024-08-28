package export

import "hcm/pkg/logs"

// BillSummaryMainTableHeader 账单调整导出表头
var BillSummaryMainTableHeader []string

var _ Table = (*BillSummaryMainTable)(nil)

func init() {
	var err error
	BillSummaryMainTableHeader, err = BillSummaryMainTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("bill adjustment table header init failed: %v", err)
	}
}

// BillSummaryMainTable 账单调整导出表头结构
type BillSummaryMainTable struct {
	MainAccountID   string `header:"二级账号ID"`
	MainAccountName string `header:"二级账号名称"`
	RootAccountID   string `header:"一级账号ID"`
	RootAccountName string `header:"一级账号名称"`

	BKBizName string `header:"业务"`

	CurrentMonthRMBCostSynced string `header:"已确认账单人民币（元）"`
	CurrentMonthCostSynced    string `header:"已确认账单美金（美元）"`
	CurrentMonthRMBCost       string `header:"当前账单人民币（元）"`
	CurrentMonthCost          string `header:"当前账单美金（美元）"`
}

// GetHeaderValues ...
func (b BillSummaryMainTable) GetHeaderValues() ([]string, error) {
	return parseHeaderFields(b)
}

// GetHeaders ...
func (b BillSummaryMainTable) GetHeaders() ([]string, error) {
	return parseHeader(b)
}
