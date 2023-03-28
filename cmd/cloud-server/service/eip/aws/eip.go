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

package aws

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
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/hooks/handler"
)

// Aws eip service.
type Aws struct {
	client     *client.ClientSet
	authorizer auth.Authorizer
	audit      audit.Interface
}

// NewAws init aws eip service.
func NewAws(client *client.ClientSet, authorizer auth.Authorizer, audit audit.Interface) *Aws {
	return &Aws{
		client:     client,
		authorizer: authorizer,
		audit:      audit,
	}
}

// AssociateEip associate eip.
func (a *Aws) AssociateEip(
	cts *rest.Contexts,
	basicInfo *types.CloudResourceBasicInfo,
	validHandler handler.ValidWithAuthHandler,
) (interface{}, error) {
	req := new(cloudproto.AwsEipAssociateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// TODO 判断 Eip 是否可关联

	// validate biz and authorize
	err := validHandler(cts, &handler.ValidWithAuthOption{
		Authorizer: a.authorizer, ResType: meta.Eip,
		Action: meta.Associate, BasicInfo: basicInfo,
	})
	if err != nil {
		return nil, err
	}

	operationInfo := protoaudit.CloudResourceOperationInfo{
		ResType:           enumor.EipAuditResType,
		ResID:             req.EipID,
		Action:            protoaudit.Associate,
		AssociatedResType: enumor.CvmAuditResType,
		AssociatedResID:   req.CvmID,
	}
	err = a.audit.ResOperationAudit(cts.Kit, operationInfo)
	if err != nil {
		logs.Errorf("create associate eip audit failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, a.client.HCService().Aws.Eip.AssociateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.AwsEipAssociateReq{
			AccountID: basicInfo.AccountID,
			CvmID:     req.CvmID,
			EipID:     req.EipID,
		},
	)
}

// DisassociateEip disassociate eip.
func (a *Aws) DisassociateEip(
	cts *rest.Contexts,
	basicInfo *types.CloudResourceBasicInfo,
	validHandler handler.ValidWithAuthHandler,
) (interface{}, error) {
	req := new(cloudproto.AwsEipDisassociateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// validate biz and authorize
	err := validHandler(cts, &handler.ValidWithAuthOption{
		Authorizer: a.authorizer, ResType: meta.Eip,
		Action: meta.Disassociate, BasicInfo: basicInfo,
	})
	if err != nil {
		return nil, err
	}

	rels, err := a.client.DataService().Global.ListEipCvmRel(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&datarelproto.EipCvmRelListReq{
			Filter: tools.EqualExpression("eip_id", req.EipID),
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
		AssociatedResType: enumor.CvmAuditResType,
		AssociatedResID:   cvmID,
	}
	err = a.audit.ResOperationAudit(cts.Kit, operationInfo)
	if err != nil {
		logs.Errorf("create disassociate eip audit failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, a.client.HCService().Aws.Eip.DisassociateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.AwsEipDisassociateReq{
			AccountID: basicInfo.AccountID,
			CvmID:     cvmID,
			EipID:     req.EipID,
		},
	)
}

// CreateEip ...
func (a *Aws) CreateEip(cts *rest.Contexts) (interface{}, error) {
	req := new(cloudproto.AwsEipCreateReq)
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
	err = a.authorizer.AuthorizeWithPerm(cts.Kit, authRes)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.HCService().Aws.Eip.CreateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&hcproto.AwsEipCreateReq{
			AccountID: req.AccountID,
			AwsEipCreateOption: &eip.AwsEipCreateOption{
				Region:             req.Region,
				PublicIpv4Pool:     req.PublicIpv4Pool,
				NetworkBorderGroup: req.NetworkBorderGroup,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	// 分配业务
	_, err = a.client.DataService().Global.BatchUpdateEip(
		cts.Kit.Ctx,
		cts.Kit.Header(),
		&dataproto.EipBatchUpdateReq{IDs: resp.IDs, BkBizID: bkBizID},
	)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RetrieveEip ...
func (a *Aws) RetrieveEip(cts *rest.Contexts, eipID string, cvmID string) (*cloudproto.AwsEipExtResult, error) {
	eipResp, err := a.client.DataService().Aws.RetrieveEip(cts.Kit.Ctx, cts.Kit.Header(), eipID)
	if err != nil {
		return nil, err
	}

	eipResult := &cloudproto.AwsEipExtResult{EipExtResult: eipResp, CvmID: cvmID}
	// 表示没有关联
	if cvmID == "" {
		return eipResult, nil
	}

	eipResult.InstanceType = "CVM"
	eipResult.InstanceId = converter.ValToPtr(cvmID)

	return eipResult, nil
}
