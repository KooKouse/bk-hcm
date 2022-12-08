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

package types

// TODO 删除该文件. 仅为解决循环依赖问题, 将原 dal/table/name.go 移到当前

// Tables defines all the database table
// related resources.
type Tables interface {
	TableName() Name
}

// Name is database table's name type
type Name string

// TODO 这里的集中化表名配置可以调整
const (
	// AuditTable is audit table's name
	AuditTable Name = "audit"
	// AccountTable is account table's name.
	AccountTable Name = "account"
)