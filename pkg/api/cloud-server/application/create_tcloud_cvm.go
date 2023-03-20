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

package application

import (
	"errors"
	"fmt"
	"strings"

	typecvm "hcm/pkg/adaptor/types/cvm"
	"hcm/pkg/criteria/validator"

	"github.com/TencentBlueKing/gopkg/collection/set"
)

const (
	tcloudPunctuation = "()`~!@#$%^&*-+=|{}[]:;',.?/"
)

var (
	tcloudPasswordInvalidError = errors.New("the password must include 8-30 characters, " +
		"and contain at least two of the following character sets: [a-z], [A-Z], [0-9] and [()`~!@#$%^&*-+=|{}[]:;',.?/]")
)

// TCloudCvmCreateReq ...
type TCloudCvmCreateReq struct {
	BkBizID               int64    `json:"bk_biz_id" validate:"required,min=1"`
	AccountID             string   `json:"account_id" validate:"required"`
	Region                string   `json:"region" validate:"required"`
	Zone                  string   `json:"zone" validate:"required"`
	Name                  string   `json:"name" validate:"required,min=1,max=60"`
	InstanceType          string   `json:"instance_type" validate:"required"`
	CloudImageID          string   `json:"cloud_image_id" validate:"required"`
	CloudVpcID            string   `json:"cloud_vpc_id" validate:"required"`
	CloudSubnetID         string   `json:"cloud_subnet_id" validate:"required"`
	PublicIPAssigned      bool     `json:"public_ip_assigned" validate:"required"`
	CloudSecurityGroupIDs []string `json:"cloud_security_group_ids" validate:"required,min=1"`

	SystemDisk struct {
		DiskType   typecvm.TCloudSystemDiskType `json:"disk_type" validate:"required"`
		DiskSizeGB int64                        `json:"disk_size_gb" validate:"required,min=50,max=32000"`
	} `json:"system_disk" validate:"required"`

	DataDisk []struct {
		DiskType   typecvm.TCloudDataDiskType `json:"disk_type" validate:"required"`
		DiskSizeGB int64                      `json:"disk_size_gb" validate:"required,min=20,max=32000"`
		DiskCount  int64                      `json:"disk_count" validate:"required,min=1"`
	} `json:"data_disk" validate:"required,max=20"`

	Password          string `json:"password" validate:"required"`
	ConfirmedPassword string `json:"confirmed_password" validate:"eqfield=Password"`

	InstanceChargeType typecvm.TCloudInstanceChargeType `json:"instance_charge_type" validate:"required"`

	InstanceChargePaidPeriod int64 `json:"instance_charge_paid_period" validate:"required,min=1"`
	AutoRenew                bool  `json:"auto_renew" validate:"required"`
	RequiredCount            int64 `json:"required_count" validate:"required,min=1,max=500"`

	Memo *string `json:"memo" validate:"omitempty"`
}

// Validate ...
func (req *TCloudCvmCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	// 校验系统硬盘
	if !req.isMultipleOfTen(req.SystemDisk.DiskSizeGB) {
		return fmt.Errorf("disk size[%d] should be not multiple of 10GB", req.SystemDisk.DiskSizeGB)
	}

	// 校验数据盘
	for _, d := range req.DataDisk {
		if !req.isMultipleOfTen(d.DiskSizeGB) {
			return fmt.Errorf("disk size[%d] should be not multiple of 10GB", d.DiskSizeGB)
		}
	}

	// 校验购买时长
	periods := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 24, 36, 48, 60}
	periodSet := set.NewInt64SetWithValues(periods)
	if !periodSet.Has(req.InstanceChargePaidPeriod) {
		return fmt.Errorf(
			"instance_charge_paid_period[%d] should be not in %+v",
			req.InstanceChargePaidPeriod, periods,
		)
	}

	// 校验密码是否符合要求
	if err := req.validatePassword(); err != nil {
		return err
	}

	return nil
}

func (req *TCloudCvmCreateReq) isMultipleOfTen(size int64) bool {
	return size%10 == 0
}

func (req *TCloudCvmCreateReq) validatePassword() error {
	// Linux实例密码必须8到30位，
	// 至少包括两项[a-z]，[A-Z]、[0-9] 和 [( ) ` ~ ! @ # $ % ^ & * - + = | { } [ ] : ; ' , . ? / ]中的特殊符号
	// Windows实例密码必须12到30位，
	// 至少包括三项[a-z]，[A-Z]，[0-9] 和 [( ) ` ~ ! @ # $ % ^ & * - + = | { } [ ] : ; ' , . ? /]中的特殊符号

	// TODO: window限制比Linux严格，Linux使用较多，先以Linux为主判断，待后续可判断系统类型再区分校验
	//  这里即使不判断，后面也会通过DryRun方式直接请求云上API校验
	if len(req.Password) < 8 || len(req.Password) > 30 {
		return fmt.Errorf("length of password should be between 8 to 30")
	}

	// 满足的规定项数量
	satisfiedCount := 0
	if strings.ContainsAny(req.Password, asciiLowercase) {
		satisfiedCount += 1
	}
	if strings.ContainsAny(req.Password, asciiUppercase) {
		satisfiedCount += 1
	}
	if strings.ContainsAny(req.Password, digits) {
		satisfiedCount += 1
	}
	if strings.ContainsAny(req.Password, tcloudPunctuation) {
		satisfiedCount += 1
	}

	// 至少满足两项
	if satisfiedCount < 2 {
		return tcloudPasswordInvalidError
	}

	return nil
}