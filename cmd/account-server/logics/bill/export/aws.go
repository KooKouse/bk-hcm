package export

import "hcm/pkg/logs"

// AwsBillItemHeaders is the headers of Aws bill item.
var AwsBillItemHeaders []string

func init() {
	var err error
	AwsBillItemHeaders, err = AwsBillItemTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("GetAwsHeader failed: %v", err)
	}
}

var _ Table = (*AwsBillItemTable)(nil)

// AwsBillItemTable aws账单导出表结构
type AwsBillItemTable struct {
	Site        string `header:"站点类型"`
	AccountDate string `header:"核算年月"`

	BizName string `header:"业务名称"`

	RootAccountName string `header:"一级账号名称"`
	MainAccountName string `header:"二级账号名称"`
	Region          string `header:"地域"`

	LocationName        string `header:"地区名称"`
	BillInvoiceIC       string `header:"发票ID"`
	BillEntity          string `header:"账单实体"`
	ProductCode         string `header:"产品代号"`
	ProductFamily       string `header:"服务组"`
	ProductName         string `header:"产品名称"`
	ApiOperation        string `header:"API操作"`
	ProductUsageType    string `header:"产品规格"`
	InstanceType        string `header:"实例类型"`
	ResourceId          string `header:"资源ID"`
	PricingTerm         string `header:"计费方式"`
	LineItemType        string `header:"计费类型"`
	LineItemDescription string `header:"计费说明"`
	UsageAmount         string `header:"用量"`
	PricingUnit         string `header:"单位"`
	Cost                string `header:"折扣前成本（外币）"`
	Currency            string `header:"外币种类"`
	RMBCost             string `header:"人民币成本（元）"`
	Rate                string `header:"汇率"`
}

// GetHeaders ...
func (c AwsBillItemTable) GetHeaders() ([]string, error) {
	return parseHeader(c)
}

// GetHeaderFields 获取表头对应的数据
func (c AwsBillItemTable) GetHeaderValues() ([]string, error) {
	return parseHeaderFields(c)
}
