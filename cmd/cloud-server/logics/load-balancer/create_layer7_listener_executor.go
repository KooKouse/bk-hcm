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
	"encoding/json"
	"fmt"
	"hcm/pkg/dal/dao/tools"

	actionlb "hcm/cmd/task-server/logics/action/load-balancer"
	actionflow "hcm/cmd/task-server/logics/flow"
	corelb "hcm/pkg/api/core/cloud/load-balancer"
	"hcm/pkg/api/data-service/task"
	hclb "hcm/pkg/api/hc-service/load-balancer"
	ts "hcm/pkg/api/task-server"
	"hcm/pkg/async/action"
	dataservice "hcm/pkg/client/data-service"
	taskserver "hcm/pkg/client/task-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	tableasync "hcm/pkg/dal/table/async"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

var _ ImportExecutor = (*CreateLayer7ListenerExecutor)(nil)

func newCreateLayer7ListenerExecutor(cli *dataservice.Client, taskCli *taskserver.Client,
	vendor enumor.Vendor, bkBizID int64, accountID string, regionIDs []string) *CreateLayer7ListenerExecutor {

	return &CreateLayer7ListenerExecutor{
		taskCli:             taskCli,
		basePreviewExecutor: newBasePreviewExecutor(cli, vendor, bkBizID, accountID, regionIDs),
	}
}

// CreateLayer7ListenerExecutor excel导入——创建四层监听器执行器
type CreateLayer7ListenerExecutor struct {
	*basePreviewExecutor

	taskCli     *taskserver.Client
	details     []*CreateLayer7ListenerDetail
	taskDetails []*createLayer7ListenerTaskDetail
}

// 用于记录 detail - 异步任务flow&task - 任务管理 之间的关系
type createLayer7ListenerTaskDetail struct {
	taskDetailID string
	flowID       string
	actionID     string
	*CreateLayer7ListenerDetail
}

// Execute 导入执行器的唯一入口
func (c *CreateLayer7ListenerExecutor) Execute(kt *kit.Kit, source string, rawDetails json.RawMessage) (
	string, error) {

	var err error
	err = c.unmarshalData(rawDetails)
	if err != nil {
		return "", err
	}

	err = c.validate(kt)
	if err != nil {
		logs.Errorf("validate failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	c.filter()

	taskID, err := c.buildTaskManagementAndDetails(kt, source)
	if err != nil {
		logs.Errorf("create task management and details failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	flowIDs, err := c.buildFlows(kt)
	if err != nil {
		logs.Errorf("build async flows failed, err: %v, rid: %s", err, kt.Rid)
		deleteErr := c.deleteTaskManagementAndDetails(kt, taskID)
		if deleteErr != nil {
			logs.Errorf("delete task management and details failed, err: %v, rid: %s", deleteErr, kt.Rid)
			return "", deleteErr
		}
		return "", err
	}
	err = c.updateTaskManagementAndDetails(kt, flowIDs, taskID)
	if err != nil {
		logs.Errorf("update task management and details failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	return taskID, nil
}

func (c *CreateLayer7ListenerExecutor) unmarshalData(rawDetail json.RawMessage) error {
	err := unmarshalData(rawDetail, &c.details)
	if err != nil {
		return err
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) validate(kt *kit.Kit) error {
	validator := &CreateLayer7ListenerPreviewExecutor{
		basePreviewExecutor: c.basePreviewExecutor,
		details:             c.details,
	}
	err := validator.validate(kt)
	if err != nil {
		return err
	}

	for _, detail := range c.details {
		if detail.Status == NotExecutable {
			return fmt.Errorf("record(%v) is not executable", detail)
		}
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) filter() {
	c.details = slice.Filter[*CreateLayer7ListenerDetail](c.details, func(detail *CreateLayer7ListenerDetail) bool {
		if detail.Status == Executable {
			return true
		}
		return false
	})
}

func (c *CreateLayer7ListenerExecutor) buildFlows(kt *kit.Kit) ([]string, error) {
	// group by clb
	clbToDetails := make(map[string][]*createLayer7ListenerTaskDetail)
	for _, detail := range c.taskDetails {
		clbToDetails[detail.CloudClbID] = append(clbToDetails[detail.CloudClbID], detail)
	}
	lbMap, err := getLoadBalancersMapByCloudID(kt, c.dataServiceCli, c.vendor,
		c.accountID, c.bkBizID, converter.MapKeyToSlice(clbToDetails))
	if err != nil {
		return nil, err
	}

	flowIDs := make([]string, 0, len(clbToDetails))
	for clbCloudID, details := range clbToDetails {
		lb := lbMap[clbCloudID]
		flowID, err := c.buildFlow(kt, lb.ID, details)
		if err != nil {
			logs.Errorf("build flow for clb(%s) failed, err: %v, rid: %s", clbCloudID, err, kt.Rid)
			return nil, err
		}
		flowIDs = append(flowIDs, flowID)
	}

	return flowIDs, nil
}

func (c *CreateLayer7ListenerExecutor) buildTaskManagementAndDetails(kt *kit.Kit, source string) (string, error) {
	taskID, err := c.createTaskManagement(kt, source)
	if err != nil {
		logs.Errorf("create task management failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	err = c.createTaskDetails(kt, taskID)
	if err != nil {
		logs.Errorf("create task details failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	return taskID, nil
}

func (c *CreateLayer7ListenerExecutor) createTaskDetails(kt *kit.Kit, taskID string) error {
	taskDetailsCreateReq := &task.CreateDetailReq{}
	for _, detail := range c.details {
		taskDetailsCreateReq.Items = append(taskDetailsCreateReq.Items, task.CreateDetailField{
			BkBizID:          c.bkBizID,
			TaskManagementID: taskID,
			Operation:        enumor.TaskCreateLayer7Listener,
			State:            enumor.TaskDetailInit,
			Param:            detail,
		})
	}

	result, err := c.dataServiceCli.Global.TaskDetail.Create(kt, taskDetailsCreateReq)
	if err != nil {
		return err
	}
	if len(result.IDs) != len(c.details) {
		return fmt.Errorf("create task details failed, expect created %d task details, but got %d",
			len(c.details), len(result.IDs))
	}

	for i := range result.IDs {
		taskDetail := &createLayer7ListenerTaskDetail{
			taskDetailID:               result.IDs[i],
			CreateLayer7ListenerDetail: c.details[i],
		}
		c.taskDetails = append(c.taskDetails, taskDetail)
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) createTaskManagement(kt *kit.Kit, source string) (string, error) {
	taskManagementCreateReq := &task.CreateManagementReq{
		Items: []task.CreateManagementField{
			{
				BkBizID:    c.bkBizID,
				Source:     enumor.TaskManagementSource(source),
				Vendor:     c.vendor,
				AccountID:  c.accountID,
				Resource:   enumor.TaskManagementResClb,
				State:      enumor.TaskManagementRunning,
				Operations: []enumor.TaskOperation{enumor.TaskCreateLayer7Listener},
			},
		},
	}

	result, err := c.dataServiceCli.Global.TaskManagement.Create(kt, taskManagementCreateReq)
	if err != nil {
		return "", err
	}
	if len(result.IDs) == 0 {
		return "", fmt.Errorf("create task management failed")
	}
	return result.IDs[0], nil
}

func (c *CreateLayer7ListenerExecutor) updateTaskManagementAndDetails(kt *kit.Kit, flowIDs []string,
	taskID string) error {

	if err := c.updateTaskManagement(kt, taskID, flowIDs); err != nil {
		logs.Errorf("update task management failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if err := c.updateTaskDetails(kt); err != nil {
		logs.Errorf("update task details failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) updateTaskManagement(kt *kit.Kit, taskID string, flowIDs []string) error {
	updateItem := task.UpdateTaskManagementField{
		ID:      taskID,
		FlowIDs: flowIDs,
	}
	updateReq := &task.UpdateManagementReq{
		Items: []task.UpdateTaskManagementField{updateItem},
	}
	err := c.dataServiceCli.Global.TaskManagement.Update(kt, updateReq)
	if err != nil {
		return err
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) updateTaskDetails(kt *kit.Kit) error {
	updateItems := make([]task.UpdateTaskDetailField, 0, len(c.taskDetails))
	for _, detail := range c.taskDetails {
		updateItems = append(updateItems, task.UpdateTaskDetailField{
			ID:            detail.taskDetailID,
			FlowID:        detail.flowID,
			TaskActionIDs: []string{detail.actionID},
		})
	}
	updateDetailsReq := &task.UpdateDetailReq{
		Items: updateItems,
	}
	err := c.dataServiceCli.Global.TaskDetail.Update(kt, updateDetailsReq)
	if err != nil {
		return err
	}
	return nil
}

func (c *CreateLayer7ListenerExecutor) buildFlow(kt *kit.Kit, lbID string,
	details []*createLayer7ListenerTaskDetail) (string, error) {

	_, err := checkResFlowRel(kt, c.dataServiceCli, lbID, enumor.LoadBalancerCloudResType)
	if err != nil {
		logs.Errorf("check res flow rel failed, res(%s), err: %v, rid: %s", lbID, err, kt.Rid)
		return "", err
	}

	flowTasks, err := c.buildFlowTask(lbID, details)
	if err != nil {
		logs.Errorf("build flow task failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	flowID, err := c.createFlowTask(kt, lbID, flowTasks)
	if err != nil {
		logs.Errorf("create flow task failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	err = lockResFlowStatus(kt, c.dataServiceCli, c.taskCli, lbID,
		enumor.LoadBalancerCloudResType, flowID, enumor.CreateListenerTaskType)
	if err != nil {
		logs.Errorf("lock resource flow status failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	for _, detail := range details {
		detail.flowID = flowID
	}
	return flowID, nil
}

func (c *CreateLayer7ListenerExecutor) buildFlowTask(lbID string,
	details []*createLayer7ListenerTaskDetail) ([]ts.CustomFlowTask, error) {

	switch c.vendor {
	case enumor.TCloud:
		return c.buildTCloudFlowTask(lbID, details)
	default:
		return nil, fmt.Errorf("vendor %s not supported", c.vendor)
	}
}

func (c *CreateLayer7ListenerExecutor) buildTCloudFlowTask(lbID string,
	details []*createLayer7ListenerTaskDetail) ([]ts.CustomFlowTask, error) {

	result := make([]ts.CustomFlowTask, 0)
	actionIDGenerator := newActionIDGenerator(1, 10)
	for _, taskDetails := range slice.Split(details, constant.BatchTaskMaxLimit) {
		cur, prev := actionIDGenerator()

		managementDetailIDs := make([]string, 0, len(taskDetails))
		listeners := make([]*hclb.TCloudListenerCreateReq, 0, len(taskDetails))
		for _, detail := range taskDetails {
			req := &hclb.TCloudListenerCreateReq{
				Name:        fmt.Sprintf("%s-%d", detail.Protocol, detail.ListenerPorts[0]),
				BkBizID:     c.bkBizID,
				LbID:        lbID,
				Protocol:    detail.Protocol,
				Port:        int64(detail.ListenerPorts[0]),
				SniSwitch:   0,
				Certificate: &corelb.TCloudCertificateInfo{},
			}

			if detail.SSLMode != "" {
				req.Certificate.SSLMode = converter.ValToPtr(detail.SSLMode)
			}
			if detail.CACloudID != "" {
				req.Certificate.CaCloudID = converter.ValToPtr(detail.CACloudID)
			}
			if len(detail.CertCloudIDs) > 0 {
				req.Certificate.CertCloudIDs = detail.CertCloudIDs
			}

			if len(detail.ListenerPorts) > 1 {
				req.EndPort = converter.ValToPtr(int64(detail.ListenerPorts[1]))
			}
			listeners = append(listeners, req)
			managementDetailIDs = append(managementDetailIDs, detail.taskDetailID)
		}

		tmpTask := ts.CustomFlowTask{
			ActionID:   action.ActIDType(cur),
			ActionName: enumor.ActionBatchTaskTCloudCreateListener,
			Params: &actionlb.BatchTaskTCloudCreateListenerOption{
				ManagementDetailIDs: managementDetailIDs,
				Listeners:           listeners,
			},
			Retry: tableasync.NewRetryWithPolicy(3, 100, 200),
		}
		if prev != "" {
			tmpTask.DependOn = []action.ActIDType{action.ActIDType(prev)}
		}
		result = append(result, tmpTask)

		for _, detail := range taskDetails {
			detail.actionID = cur
		}
	}
	return result, nil
}

func (c *CreateLayer7ListenerExecutor) createFlowTask(kt *kit.Kit, lbID string,
	flowTasks []ts.CustomFlowTask) (string, error) {

	addReq := &ts.AddCustomFlowReq{
		Name: enumor.FlowLoadBalancerCreateListener,
		ShareData: tableasync.NewShareData(map[string]string{
			"lb_id": lbID,
		}),
		Tasks:       flowTasks,
		IsInitState: true,
	}
	result, err := c.taskCli.CreateCustomFlow(kt, addReq)
	if err != nil {
		logs.Errorf("call taskserver to batch add rs custom flow failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	flowID := result.ID
	// 从Flow，负责监听主Flow的状态
	flowWatchReq := &ts.AddTemplateFlowReq{
		Name: enumor.FlowLoadBalancerOperateWatch,
		Tasks: []ts.TemplateFlowTask{{
			ActionID: "1",
			Params: &actionflow.LoadBalancerOperateWatchOption{
				FlowID:   flowID,
				ResID:    lbID,
				ResType:  enumor.LoadBalancerCloudResType,
				TaskType: enumor.CreateListenerTaskType,
			},
		}},
	}
	_, err = c.taskCli.CreateTemplateFlow(kt, flowWatchReq)
	if err != nil {
		logs.Errorf("call taskserver to create res flow status watch task failed, err: %v, flowID: %s, rid: %s",
			err, flowID, kt.Rid)
		return "", err
	}

	return flowID, nil
}

func (c *CreateLayer7ListenerExecutor) deleteTaskManagementAndDetails(kt *kit.Kit, taskID string) error {
	deleteReq := &task.DeleteManagementReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("id", taskID),
		),
	}
	err := c.dataServiceCli.Global.TaskManagement.Delete(kt, deleteReq)
	if err != nil {
		logs.Errorf("delete task management failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	taskDetailIDs := make([]string, 0, len(c.taskDetails))
	for _, detail := range c.taskDetails {
		taskDetailIDs = append(taskDetailIDs, detail.taskDetailID)
	}
	deleteDetailsReq := &task.DeleteDetailReq{
		Filter: tools.ExpressionAnd(
			tools.RuleIn("id", taskDetailIDs),
		),
	}
	err = c.dataServiceCli.Global.TaskDetail.Delete(kt, deleteDetailsReq)
	if err != nil {
		logs.Errorf("delete task details failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	return nil
}