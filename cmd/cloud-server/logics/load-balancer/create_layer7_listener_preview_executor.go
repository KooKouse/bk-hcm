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

package lblogic

import (
	"fmt"
	"reflect"
	"strings"

	"hcm/pkg/api/core"
	corelb "hcm/pkg/api/core/cloud/load-balancer"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

var _ ImportPreviewExecutor = (*CreateLayer7ListenerPreviewExecutor)(nil)

func newCreateLayer7ListenerPreviewExecutor(cli *dataservice.Client, vendor enumor.Vendor, bkBizID int64,
	accountID string, regionIDs []string) *CreateLayer7ListenerPreviewExecutor {

	return &CreateLayer7ListenerPreviewExecutor{
		basePreviewExecutor: newBasePreviewExecutor(cli, vendor, bkBizID, accountID, regionIDs),
	}
}

// CreateLayer7ListenerPreviewExecutor 创建七层监听器预览执行器
type CreateLayer7ListenerPreviewExecutor struct {
	*basePreviewExecutor

	details []*CreateLayer7ListenerDetail
}

// Execute 导入执行器的唯一入口
func (c *CreateLayer7ListenerPreviewExecutor) Execute(kt *kit.Kit, rawData [][]string) (interface{}, error) {
	err := c.convertDataToPreview(rawData)
	if err != nil {
		logs.Errorf("convert create listener layer7 data failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	err = c.validate(kt)
	if err != nil {
		logs.Errorf("validate create listener layer7 data failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return c.details, nil
}

var createLayer7ListenerDetailRefType = reflect.TypeOf(CreateLayer7ListenerDetail{})

func (c *CreateLayer7ListenerPreviewExecutor) convertDataToPreview(rawData [][]string) error {
	for _, data := range rawData {
		data = trimSpaceForSlice(data)
		if len(data) > 8 {
			return fmt.Errorf("excel table data format error")
		}

		detail := &CreateLayer7ListenerDetail{}
		for i, value := range data {
			field := createLayer7ListenerDetailRefType.Field(i)
			fieldValue := reflect.ValueOf(detail).Elem().FieldByName(field.Name)
			if fieldValue.Type().Kind() == reflect.String {
				fieldValue.SetString(value)
				continue
			}
			switch field.Name {
			case "ListenerPorts":
				ports, err := parsePort(value)
				if err != nil {
					return err
				}
				detail.ListenerPorts = ports
			case "CertCloudIDs":
				certStr := strings.TrimRight(strings.TrimLeft(value, "["), "]")
				for _, s := range strings.Split(certStr, ",") {
					cur := strings.TrimSpace(s)
					if len(cur) == 0 {
						continue
					}
					detail.CertCloudIDs = append(detail.CertCloudIDs, cur)
				}
			}
		}

		detail.Protocol = enumor.ProtocolType(strings.ToUpper(string(detail.Protocol)))
		c.details = append(c.details, detail)
	}

	return nil
}

func (c *CreateLayer7ListenerPreviewExecutor) validate(kt *kit.Kit) error {

	recordMap := make(map[string]int)
	clbIDMap := make(map[string]struct{})
	for cur, detail := range c.details {
		detail.validate()
		// 检查记录是否重复
		key := fmt.Sprintf("%s-%s-%v", detail.CloudClbID, detail.Protocol, detail.ListenerPorts)
		if i, ok := recordMap[key]; ok {
			c.details[i].Status = NotExecutable
			c.details[i].ValidateResult += fmt.Sprintf("存在重复记录, line: %d;", i+1)

			detail.Status = NotExecutable
			detail.ValidateResult += fmt.Sprintf("存在重复记录, line: %d;", cur+1)
		}
		recordMap[key] = cur
		clbIDMap[detail.CloudClbID] = struct{}{}
	}
	err := c.validateWithDB(kt, converter.MapKeyToSlice(clbIDMap))
	if err != nil {
		logs.Errorf("validate with db failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	return nil
}

func (c *CreateLayer7ListenerPreviewExecutor) validateWithDB(kt *kit.Kit, cloudIDs []string) error {
	lbMap, err := getLoadBalancersMapByCloudID(kt, c.dataServiceCli, c.vendor, c.accountID, c.bkBizID, cloudIDs)
	if err != nil {
		return err
	}

	for _, detail := range c.details {
		lb, ok := lbMap[detail.CloudClbID]
		if !ok {
			return fmt.Errorf("clb(%s) not exist", detail.CloudClbID)
		}
		if _, ok = c.regionIDMap[lb.Region]; !ok {
			return fmt.Errorf("clb region not match, clb.region: %s, input: %v", lb.Region, c.regionIDMap)
		}

		ipSet := append(lb.PrivateIPv4Addresses, lb.PrivateIPv6Addresses...)
		ipSet = append(ipSet, lb.PublicIPv4Addresses...)
		ipSet = append(ipSet, lb.PublicIPv6Addresses...)
		if detail.ClbVipDomain != lb.Domain && !slice.IsItemInSlice(ipSet, detail.ClbVipDomain) {
			detail.Status = NotExecutable
			detail.ValidateResult += fmt.Sprintf("clb的vip(%s)不匹配", detail.ClbVipDomain)
		}

		if err = c.validateListener(kt, detail); err != nil {
			logs.Errorf("validate listener failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}
	}
	return nil
}

func (c *CreateLayer7ListenerPreviewExecutor) validateListener(kt *kit.Kit, detail *CreateLayer7ListenerDetail) error {

	switch c.vendor {
	case enumor.TCloud:
		listeners, err := c.getTCloudListenersByPort(kt, detail.CloudClbID, detail.ListenerPorts[0])
		if err != nil {
			return err
		}
		if len(listeners) == 0 {
			return nil
		}
		return c.validateTCloudListener(kt, detail, listeners)
	default:
		return fmt.Errorf("vendor(%s) not support", c.vendor)
	}
}

func (c *CreateLayer7ListenerPreviewExecutor) validateTCloudListener(kt *kit.Kit, detail *CreateLayer7ListenerDetail,
	listeners []corelb.Listener[corelb.TCloudListenerExtension]) error {

	for _, listener := range listeners {
		if listener.Protocol == enumor.UdpProtocol {
			// udp协议可以和 http/https公用端口
			continue
		}

		if detail.Status != NotExecutable {
			detail.Status = Existing
		}

		if listener.SniSwitch == enumor.SniTypeOpen {
			// sni开启时, 将不做重复检查, 直接返回不可执行
			detail.Status = NotExecutable
			detail.ValidateResult += fmt.Sprintf("clb(%s)的监听器(%d)已存在并开启SNI",
				listener.CloudLbID, detail.ListenerPorts[0])
			return nil
		}

		if listener.Protocol != detail.Protocol {
			detail.Status = NotExecutable
			detail.ValidateResult += fmt.Sprintf("clb(%s)的监听器(%d)已存在, 且协议不匹配",
				listener.CloudLbID, detail.ListenerPorts[0])
			return nil
		}

		if detail.Protocol == enumor.HttpProtocol {
			return nil
		}

		err := c.validateTCloudCert(kt, listener.Extension.Certificate, detail, listener.CloudLbID, listener.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CreateLayer7ListenerPreviewExecutor) validateTCloudCert(kt *kit.Kit,
	listenerCert *corelb.TCloudCertificateInfo, curDetail *CreateLayer7ListenerDetail,
	cloudLBID, lblID string) error {

	if listenerCert == nil {
		logs.Errorf("listener(%s) cert is nil, rid: %s", lblID, kt.Rid)
		return fmt.Errorf("listner(%s) cert is nil", lblID)
	}
	if converter.PtrToVal(listenerCert.SSLMode) != curDetail.SSLMode ||
		converter.PtrToVal(listenerCert.CaCloudID) != curDetail.CACloudID ||
		len(listenerCert.CertCloudIDs) != len(curDetail.CertCloudIDs) {

		curDetail.Status = NotExecutable
		curDetail.ValidateResult += fmt.Sprintf("clb(%s)的监听器(%d)已存在, 且证书信息不匹配",
			cloudLBID, curDetail.ListenerPorts[0])
		return nil
	}

	for _, id := range curDetail.CertCloudIDs {
		if !slice.IsItemInSlice(listenerCert.CertCloudIDs, id) {
			curDetail.Status = NotExecutable
			curDetail.ValidateResult += fmt.Sprintf("clb(%s)的监听器(%d)已存在, 且证书信息不匹配",
				cloudLBID, curDetail.ListenerPorts[0])
			return nil
		}
	}
	return nil
}

func (c *CreateLayer7ListenerPreviewExecutor) getTCloudListenersByPort(kt *kit.Kit, lbCloudID string, port int) (
	[]corelb.Listener[corelb.TCloudListenerExtension], error) {

	req := &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("account_id", c.accountID),
			tools.RuleEqual("bk_biz_id", c.bkBizID),
			tools.RuleEqual("cloud_lb_id", lbCloudID),
			tools.RuleEqual("port", port),
			tools.RuleEqual("vendor", c.vendor),
		),
		Page: core.NewDefaultBasePage(),
	}
	resp, err := c.dataServiceCli.TCloud.LoadBalancer.ListListener(kt, req)
	if err != nil {
		logs.Errorf("list listener failed, port: %d, cloudLBID: %s, err: %v, rid: %s",
			port, lbCloudID, err, kt.Rid)
		return nil, err
	}
	if len(resp.Details) > 0 {
		return resp.Details, nil
	}
	return nil, nil
}

// CreateLayer7ListenerDetail 创建七层监听器预览记录
type CreateLayer7ListenerDetail struct {
	ClbVipDomain string `json:"clb_vip_domain"`
	CloudClbID   string `json:"cloud_clb_id"`

	Protocol      enumor.ProtocolType `json:"protocol"`
	ListenerPorts []int               `json:"listener_port"`
	SSLMode       string              `json:"ssl_mode"`
	CertCloudIDs  []string            `json:"cert_cloud_ids"`
	CACloudID     string              `json:"ca_cloud_id"`
	UserRemark    string              `json:"user_remark"`

	Status         ImportStatus `json:"status"`
	ValidateResult string       `json:"validate_result"`
}

func (c *CreateLayer7ListenerDetail) validate() {
	err := validatePort(c.ListenerPorts)
	if err != nil {
		c.Status = NotExecutable
		c.ValidateResult = err.Error()
		return
	}

	switch c.Protocol {
	case enumor.HttpProtocol:
		if len(c.SSLMode) > 0 || len(c.CertCloudIDs) > 0 || len(c.CACloudID) > 0 {
			c.Status = NotExecutable
			c.ValidateResult = "HTTP协议不支持填写证书信息"
			return
		}
	case enumor.HttpsProtocol:
		if len(c.SSLMode) == 0 || len(c.CertCloudIDs) == 0 {
			c.Status = NotExecutable
			c.ValidateResult = "HTTPS协议必须填写证书信息"
			return
		}
		if c.SSLMode != "MUTUAL" && c.SSLMode != "UNIDIRECTIONAL" {
			c.Status = NotExecutable
			c.ValidateResult = "证书认证方式错误"
			return
		}
	default:
		c.Status = NotExecutable
		c.ValidateResult = "协议类型错误"
		return
	}

	c.Status = Executable
}
