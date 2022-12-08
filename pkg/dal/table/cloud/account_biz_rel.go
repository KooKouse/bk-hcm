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

package cloud

import (
	"time"

	"hcm/pkg/dal/dao/types"
	"hcm/pkg/dal/table"
	"hcm/pkg/runtime/filter"
)

// AccountBizRelTable 云账户与业务关联表
type AccountBizRelTable struct {
	// 账号自增 ID
	ID uint64 `db:"id"`
	// 蓝鲸业务 ID
	BkBizID int `db:"bk_biz_id"`
	// 云账号主键 ID
	AccountID uint64 `db:"account_id"`
	// 创建者
	Creator string `db:"creator"`
	// 更新者
	Reviser string `db:"reviser"`
	// 创建时间
	CreatedAt *time.Time `db:"created_at"`
	// 更新时间
	UpdatedAt *time.Time `db:"updated_at"`
	// table manager
	TableManager *table.TableManager
}

var _ table.Table = new(AccountBizRelTable)

// TableName ...
func (t *AccountBizRelTable) TableName() string {
	return "account_biz_rel"
}

// SQLForInsert ...
func (t *AccountBizRelTable) SQLForInsert() string {
	return t.TableManager.SQLForInsert(t)
}

// SQLForUpdate ...
func (t *AccountBizRelTable) SQLForUpdate(expr *filter.Expression) (string, error) {
	return t.TableManager.SQLForUpdate(t, expr)
}

// FieldKVForUpdate ...
func (t *AccountBizRelTable) FieldKVForUpdate() map[string]interface{} {
	return t.TableManager.FieldKVForUpdate(t)
}

// SQLForList ...
func (t *AccountBizRelTable) SQLForList(opt *types.ListOption, whereOpt *filter.SQLWhereOption) (string, error) {
	return t.TableManager.SQLForList(t, opt, whereOpt)
}

// SQLForDelete ...
func (t *AccountBizRelTable) SQLForDelete(expr *filter.Expression) (string, error) {
	return t.TableManager.SQLForDelete(t, expr)
}