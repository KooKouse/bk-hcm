package export

import "hcm/pkg/logs"

// BillSummaryRootTableHeader 账单调整导出表头
var BillSummaryRootTableHeader []string

var _ Table = (*BillSummaryRootTable)(nil)

func init() {
	var err error
	BillSummaryRootTableHeader, err = BillSummaryRootTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("bill adjustment table header init failed: %v", err)
	}
}

// BillSummaryRootTable 账单调整导出表头结构
type BillSummaryRootTable struct {
	RootAccountID             string `header:"一级账号ID"`
	RootAccountName           string `header:"一级账号名称"`
	State                     string `header:"账号状态"`
	CurrentMonthRMBCostSynced string `header:"账单同步（人民币-元）当月"`
	LastMonthRMBCostSynced    string `header:"账单同步（人民币-元）上月"`
	CurrentMonthCostSynced    string `header:"账单同步（美金-美元）当月"`
	LastMonthCostSynced       string `header:"账单同步（美金-美元）上月"`
	MonthOnMonthValue         string `header:"账单同步环比"`
	CurrentMonthRMB           string `header:"当前账单人民币（元）"`
	CurrentMonthCost          string `header:"当前账单美金（美元）"`
	AdjustRMBCost             string `header:"调账人民币（元）"`
	AdjustCost                string `header:"调账美金（美元）"`
}

// GetHeaderFields ...
func (b BillSummaryRootTable) GetHeaderFields() ([]string, error) {
	return parseHeaderFields(b)
}

// GetHeaders ...
func (b BillSummaryRootTable) GetHeaders() ([]string, error) {
	return parseHeader(b)
}
