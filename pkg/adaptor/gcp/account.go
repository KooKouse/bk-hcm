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

	"hcm/pkg/adaptor/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

var _ types.AccountInterface = new(gcp)

// AccountCheck check account authentication information and permissions.
// TODO: 仅用于测试
func (g *gcp) AccountCheck(kt *kit.Kit, secret *types.Secret) error {
	client, err := g.computeClient(kt, secret)
	if err != nil {
		return fmt.Errorf("init gcp client failed, err: %v", err)
	}

	_, err = client.Regions.List(secret.ProjectID).Context(kt.Ctx).Do()
	if err != nil {
		logs.Errorf("describe regions failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}