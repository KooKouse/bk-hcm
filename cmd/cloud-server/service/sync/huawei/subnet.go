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

package huawei

import (
	"time"

	"hcm/pkg/api/core"
	"hcm/pkg/api/hc-service/sync"
	dataservice "hcm/pkg/client/data-service"
	hcservice "hcm/pkg/client/hc-service"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
)

// SyncSubnet ...
func SyncSubnet(kt *kit.Kit, hcCli *hcservice.Client, dataCli *dataservice.Client, accountID string,
	regions []string) error {

	start := time.Now()
	logs.V(3).Infof("huawei account[%s] sync subnet start, time: %v, rid: %s", accountID, start, kt.Rid)

	defer func() {
		logs.V(3).Infof("huawei account[%s] sync subnet end, cost: %v, rid: %s", accountID, time.Since(start), kt.Rid)
	}()

	for _, region := range regions {
		listReq := &core.ListReq{
			Filter: &filter.Expression{
				Op: filter.And,
				Rules: []filter.RuleFactory{
					&filter.AtomRule{
						Field: "account_id",
						Op:    filter.Equal.Factory(),
						Value: accountID,
					},
					&filter.AtomRule{
						Field: "region",
						Op:    filter.Equal.Factory(),
						Value: region,
					},
				},
			},
			Page: &core.BasePage{
				Start: 0,
				Limit: core.DefaultMaxPageLimit,
			},
			Fields: []string{"cloud_id"},
		}
		startIndex := uint32(0)
		for {
			listReq.Page.Start = startIndex

			vpcResult, err := dataCli.Global.Vpc.List(kt.Ctx, kt.Header(), listReq)
			if err != nil {
				logs.Errorf("list huawei vpc failed, err: %v, rid: %s", err, kt.Rid)
				return err
			}

			for _, vpc := range vpcResult.Details {
				req := &sync.HuaWeiSubnetSyncReq{
					AccountID:  accountID,
					Region:     region,
					CloudVpcID: vpc.CloudID,
				}
				if err = hcCli.HuaWei.Subnet.SyncSubnet(kt.Ctx, kt.Header(), req); Error(err) != nil {
					logs.Errorf("sync huawei subnet failed, err: %v, req: %v, rid: %s", err, req, kt.Rid)
					return err
				}
			}

			if len(vpcResult.Details) < int(core.DefaultMaxPageLimit) {
				break
			}

			startIndex += uint32(core.DefaultMaxPageLimit)
		}
	}

	return nil
}