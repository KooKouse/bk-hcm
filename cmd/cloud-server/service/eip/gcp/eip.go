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

package gcp

import (
	"fmt"

	"hcm/cmd/cloud-server/logics/audit"
	"hcm/pkg/adaptor/types/eip"
	cloudproto "hcm/pkg/api/cloud-server/eip"
	"hcm/pkg/api/core"
	protoaudit "hcm/pkg/api/data-service/audit"
	datarelproto "hcm/pkg/api/data-service/cloud"
	dataproto "hcm/pkg/api/data-service/cloud/eip"
	hcproto "hcm/pkg/api/hc-service/eip"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/hooks/handler"
)

// Gcp eip service.
type Gcp struct {
	client     *client.ClientSet
	authorizer auth.Authorizer
	audit      audit.Interface
}

// NewGcp init gcp eip service.
func NewGcp(client *client.ClientSet, authorizer auth.Authorizer, audit audit.Interface) *Gcp {
	return &Gcp{
		client:     client,
		authorizer: authorizer,
		audit:      audit,
	}
}

// AssociateEip associate eip.
func (g *Gcp) AssociateEip(
	cts *rest.Contexts,
	basicInfo *types.CloudResourceBasicInfo,
	validHandler handler.ValidWithAuthHandler,
) (interface{}, error) {
	req := new(cloudproto.GcpEipAssociateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// TODO 判断 Eip 是否可关联

	// validate biz and authorize
	err := validHandler(cts, &handler.ValidWithAuthOption{
		Authorizer: g.authorizer, ResType: meta.Eip,
		Action: meta.Associate, BasicInfo: basicInfo,
	})
	if err != nil {
		return nil, err
	}

	operationInfo := protoaudit.CloudResourceOperationInfo{
		ResType:           enumor.EipAuditResType,
		ResID:             req.EipID,
		Action:            protoaudit.Associate,
		AssociatedResType: enumor.NetworkInterfaceAuditResType,
		AssociatedResID:   req.NetworkInterfaceID,
	}
	err = g.audit.ResOperationAudit(cts.Kit, operationInfo)
	if err != nil {
		logs.Errorf("create associate eip audit failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, g.client.HCService().Gcp.Eip.AssociateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.GcpEipAssociateReq{
			AccountID:          basicInfo.AccountID,
			CvmID:              req.CvmID,
			EipID:              req.EipID,
			NetworkInterfaceID: req.NetworkInterfaceID,
		},
	)
}

// DisassociateEip disassociate eip.
func (g *Gcp) DisassociateEip(
	cts *rest.Contexts,
	basicInfo *types.CloudResourceBasicInfo,
	validHandler handler.ValidWithAuthHandler,
) (interface{}, error) {
	req := new(cloudproto.GcpEipDisassociateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// validate biz and authorize
	err := validHandler(cts, &handler.ValidWithAuthOption{
		Authorizer: g.authorizer, ResType: meta.Eip,
		Action: meta.Disassociate, BasicInfo: basicInfo,
	})
	if err != nil {
		return nil, err
	}

	rels, err := g.client.DataService().Global.ListEipCvmRel(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&datarelproto.EipCvmRelListReq{
			Filter: tools.ContainersExpression("eip_id", []string{req.EipID}),
			Page:   core.DefaultBasePage,
		},
	)
	if len(rels.Details) == 0 {
		return nil, fmt.Errorf("eip(%s) not associated", req.EipID)
	}

	cvmID := rels.Details[0].CvmID

	operationInfo := protoaudit.CloudResourceOperationInfo{
		ResType:           enumor.EipAuditResType,
		ResID:             req.EipID,
		Action:            protoaudit.Disassociate,
		AssociatedResType: enumor.NetworkInterfaceAuditResType,
		AssociatedResID:   req.NetworkInterfaceID,
	}
	err = g.audit.ResOperationAudit(cts.Kit, operationInfo)
	if err != nil {
		logs.Errorf("create disassociate eip audit failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, g.client.HCService().Gcp.Eip.DisassociateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.GcpEipDisassociateReq{
			AccountID:          basicInfo.AccountID,
			CvmID:              cvmID,
			EipID:              req.EipID,
			NetworkInterfaceID: req.NetworkInterfaceID,
		},
	)
}

// CreateEip ...
func (g *Gcp) CreateEip(cts *rest.Contexts) (interface{}, error) {
	req := new(cloudproto.GcpEipCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	bkBizID, err := cts.PathParameter("bk_biz_id").Uint64()
	if err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	// validate biz and authorize
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Eip, Action: meta.Create}, BizID: int64(bkBizID)}
	err = g.authorizer.AuthorizeWithPerm(cts.Kit, authRes)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.HCService().Gcp.Eip.CreateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.GcpEipCreateReq{
			AccountID: req.AccountID,
			GcpEipCreateOption: &eip.GcpEipCreateOption{
				EipName:     req.EipName,
				Region:      req.Region,
				NetworkTier: req.NetworkTier,
				IpVersion:   req.IpVersion,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	// 分配业务
	_, err = g.client.DataService().Global.BatchUpdateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&dataproto.EipBatchUpdateReq{IDs: resp.IDs, BkBizID: bkBizID},
	)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
