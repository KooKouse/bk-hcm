import { computed, defineComponent, reactive, ref, watch } from 'vue';
import { Button, Checkbox, Form, Input, Popover, Radio, Select, Slider, Table } from 'bkui-vue';
import { BkRadioButton, BkRadioGroup } from 'bkui-vue/lib/radio';
import { Plus } from 'bkui-vue/lib/icon';
import './index.scss';
import { ResourceTypeEnum, VendorEnum } from '@/common/constant';
import { useTable } from '@/hooks/useTable/useTable';
import DetailHeader from '@/views/resource/resource-manage/common/header/detail-header';
import ConditionOptions from '../components/common/condition-options.vue';
import CommonCard from '@/components/CommonCard';
import ZoneSelector from '@/components/zone-selector/index.vue';
import VpcSelector from '../components/common/vpc-selector';
import InputNumber from '@/components/input-number';
import SubnetSelector from '../components/common/subnet-selector';
import SubnetPreviewDialog, { ISubnetItem } from '../cvm/children/SubnetPreviewDialog';
import VpcPreviewDialog from '../cvm/children/VpcPreviewDialog';
import PrimaryStandZoneSelector from '../components/common/primary-stand-zone-selector';
import CommonDialog from '@/components/common-dialog';

const { FormItem } = Form;
const { Option } = Select;
const { Column } = Table;

export default defineComponent({
  name: 'ApplyLoadBalancer',
  setup() {
    const formModel = reactive({
      bizId: '' as string,
      cloudAccountId: '' as string, // 云账号
      accountType: 'standard' as 'traditional' | 'standard', // 账户类型
      vendor: null as VendorEnum, // 云厂商
      region: '' as string, // 云地域
      netType: 'public' as 'public' | 'intra', // 网络类型
      ipType: 'ipv4' as 'ipv4' | 'ipv6' | 'ipv6_nat64', // IP类型
      zoneType: 'single' as 'single' | 'primaryStand', // 可用区类型
      zone: '' as string | string[], // 可用区
      cloud_vpc_id: '' as string, // 所属的VPC网络
      cloud_subnet_id: '' as string, // 所属的子网
      clbSpecType: '' as string, // CLB规格类型
      carrierType: '1' as string, // 运营商类型
      eip_id: '' as string, // 弹性IP
      instanceChargeType: '包年包月' as string, // 实例计费类型
      networkChargeType: '1' as string, // 网络计费类型
      bandwidth: 0 as number, // 带宽上限
      quantity: 1 as number, // 购买数量
      duration: 1 as number, // 购买时长
      autoRenewal: false, // 自动续费
      name: '', // 名称
      memo: '', // 实例备注
      remark: '', // 申请单备注
    });
    const isIntranet = computed(() => formModel.netType === 'intra');
    const vpcId = ref('');
    const vpcData = ref(null);
    const subnetData = ref(null);
    const isVpcPreviewDialogShow = ref(false);
    const isSubnetPreviewDialogShow = ref(false);

    const handleZoneChange = () => {
      vpcId.value = '';
      formModel.cloud_vpc_id = '';
      formModel.cloud_subnet_id = '';
    };
    const handleVpcChange = (vpc: any) => {
      console.log(vpc);
      vpcData.value = vpc;
      if (vpcId.value !== vpc.id) {
        vpcId.value = vpc.id;
        formModel.cloud_subnet_id = '';
      }
    };
    const handleSubnetDataChange = (data: ISubnetItem) => {
      console.log(data);
      subnetData.value = data;
    };
    const formItemOptions = computed(() => [
      {
        id: 'config',
        title: '配置信息',
        children: [
          [
            {
              label: '网络类型',
              required: true,
              property: 'netType',
              content: () => (
                <BkRadioGroup v-model={formModel.netType}>
                  <BkRadioButton label='public' class='w120'>
                    公网
                  </BkRadioButton>
                  <BkRadioButton label='intra' class='w120'>
                    内网
                  </BkRadioButton>
                </BkRadioGroup>
              ),
            },
            {
              label: 'IP版本',
              required: true,
              property: 'ipType',
              hidden: isIntranet.value,
              content: () => (
                <BkRadioGroup v-model={formModel.ipType}>
                  <BkRadioButton label='ipv4' class='w120'>
                    IPv4
                  </BkRadioButton>
                  <BkRadioButton label='ipv6' class='w120'>
                    Ipv6
                  </BkRadioButton>
                  <BkRadioButton label='ipv6_nat64' class='w120'>
                    Ipv6 NAT64
                  </BkRadioButton>
                </BkRadioGroup>
              ),
            },
          ],
          [
            {
              label: '可用区类型',
              required: true,
              property: 'zoneType',
              hidden: isIntranet.value,
              content: () => (
                <BkRadioGroup v-model={formModel.zoneType}>
                  <BkRadioButton label='single' class='w120'>
                    单可用区
                  </BkRadioButton>
                  <BkRadioButton label='primaryStand' class='w120'>
                    主备可用区
                  </BkRadioButton>
                </BkRadioGroup>
              ),
            },
            {
              label: '可用区',
              required: true,
              property: 'zone',
              content: () => {
                let zoneSelectorVNode = null;
                if (isIntranet.value) {
                  zoneSelectorVNode = <div>多选可用区</div>;
                } else {
                  if (formModel.zoneType === 'single') {
                    zoneSelectorVNode = (
                      <ZoneSelector
                        v-model={formModel.zone}
                        vendor={formModel.vendor}
                        region={formModel.region}
                        onChange={handleZoneChange}
                        delayed={true}
                      />
                    );
                  } else {
                    zoneSelectorVNode = (
                      <PrimaryStandZoneSelector
                        v-model={formModel.zone}
                        vendor={formModel.vendor}
                        region={formModel.region}
                      />
                    );
                  }
                }
                return zoneSelectorVNode;
              },
            },
          ],
          {
            label: 'VPC',
            required: true,
            property: 'cloud_vpc_id',
            content: () => (
              <div class='component-with-preview'>
                <VpcSelector
                  class='base'
                  v-model={formModel.cloud_vpc_id}
                  bizId={formModel.bizId}
                  accountId={formModel.cloudAccountId}
                  vendor={formModel.vendor}
                  region={formModel.region}
                  zone={formModel.zone}
                  onChange={handleVpcChange}
                />
                <Button
                  class='preview-btn'
                  text
                  theme='primary'
                  disabled={!formModel.cloud_vpc_id}
                  onClick={() => (isVpcPreviewDialogShow.value = true)}>
                  预览
                </Button>
              </div>
            ),
          },
          {
            label: '子网',
            required: true,
            property: 'cloud_subnet_id',
            hidden: !isIntranet.value,
            content: () => (
              <div class='component-with-preview'>
                <SubnetSelector
                  class='base'
                  v-model={formModel.cloud_subnet_id}
                  bizId={formModel.bizId}
                  vpcId={vpcId.value}
                  vendor={formModel.vendor}
                  region={formModel.region}
                  accountId={formModel.cloudAccountId}
                  zone={formModel.zone}
                  clearable={false}
                  handleChange={handleSubnetDataChange}
                />
                <Button
                  class='preview-btn'
                  text
                  theme='primary'
                  disabled={!formModel.cloud_subnet_id}
                  onClick={() => {
                    isSubnetPreviewDialogShow.value = true;
                  }}>
                  预览
                </Button>
              </div>
            ),
          },
          {
            label: '负载均衡规格类型',
            required: true,
            property: 'clbSpecType',
            hidden: isIntranet.value,
            content: () => (
              <BkRadioGroup v-model={formModel.clbSpecType}>
                <BkRadioButton label='' class='w120'>
                  共享型
                </BkRadioButton>
                <BkRadioButton label={selectedClbSpecType.model} class='w120'>
                  性能容量型
                </BkRadioButton>
              </BkRadioGroup>
            ),
          },
          {
            label: '运营商类型',
            required: true,
            property: 'carrierType',
            hidden: isIntranet.value,
            content: () => (
              <Select v-model={formModel.carrierType}>
                <Option label='1' value='1' />
                <Option label='2' value='2' />
              </Select>
            ),
          },
          {
            label: '弹性公网 IP',
            property: 'eip',
            hidden: isIntranet.value,
            content: () => (
              <Button
                onClick={() => (isBindEipDialogShow.value = true)}
                disabled={formModel.instanceChargeType === '包年包月'}>
                <Plus class='f24' />
                绑定弹性 IP
              </Button>
            ),
          },
        ],
      },
      {
        id: 'applyInfo',
        title: '购买信息',
        children: [
          {
            label: '实例计费模式',
            property: 'instanceChargeType',
            simpleShow: true,
            content: () => (
              <div class='simple-show-container'>
                <span class='label'>实例计费模式</span>:<span class='value'>{formModel.instanceChargeType}</span>
                <i
                  v-bk-tooltips={{ content: formModel.instanceChargeType, placement: 'right' }}
                  class='hcm-icon bkhcm-icon-prompt'></i>
              </div>
            ),
          },
          {
            label: '网络计费模式',
            required: true,
            property: 'networkChargeType',
            hidden: (formModel.netType === 'public' && formModel.accountType === 'traditional') || isIntranet.value,
            content: () => (
              <BkRadioGroup v-model={formModel.networkChargeType}>
                <BkRadioButton label='1' class='w88'>
                  包月
                </BkRadioButton>
                <BkRadioButton label='2' class='w88'>
                  按小时
                </BkRadioButton>
                <BkRadioButton label='3' class='w88'>
                  按流量
                </BkRadioButton>
                <BkRadioButton label='4' class='w88'>
                  共享带宽包
                </BkRadioButton>
              </BkRadioGroup>
            ),
          },
          {
            label: '带宽上限（Mbps）',
            required: true,
            property: 'bandwidth',
            hidden: (formModel.netType === 'public' && formModel.accountType === 'traditional') || isIntranet.value,
            content: () => (
              <div class='slider-wrap'>
                <Slider
                  v-model={formModel.bandwidth}
                  maxValue={5120}
                  step={256}
                  customContent={{
                    0: { label: '0' },
                    256: { label: '256' },
                    512: { label: '512' },
                    1024: { label: '1024' },
                    2048: { label: '2048' },
                    5120: { label: '5120' },
                  }}
                  showInput
                />
                <div class='slider-unit-suffix'>Mbps</div>
              </div>
            ),
          },
          [
            {
              label: '购买数量',
              required: true,
              property: 'quantity',
              content: () => (
                <>
                  <InputNumber v-model={formModel.quantity} min={1} />
                  <div class='quota-info'>
                    所在地域配额为 <span class='quota-number'>{130}</span> / 500
                  </div>
                </>
              ),
            },
            {
              label: '购买时长',
              required: true,
              property: 'duration',
              content: () => (
                <div class='flex-row'>
                  <Input
                    v-model={formModel.duration}
                    class='input-select-wrap'
                    type='number'
                    placeholder='0'
                    min={1}
                    max={unit.value === 'month' ? 11 : 5}>
                    {{
                      suffix: () => (
                        <Select v-model={unit.value} clearable={false} class='input-suffix-select'>
                          <Option label='月' value='month' />
                          <Option label='年' value='year' />
                        </Select>
                      ),
                    }}
                  </Input>
                  <Checkbox class='ml24' v-model={formModel.autoRenewal}>
                    自动续费
                  </Checkbox>
                </div>
              ),
            },
          ],
          {
            label: '实例名称',
            required: true,
            property: 'name',
            content: () => (
              <div class='flex-row'>
                <Input class='w500' v-model={formModel.name}></Input>
                <Checkbox class='ml24' v-model={formModel.memo}>
                  实例备注
                </Checkbox>
              </div>
            ),
          },
          {
            label: '申请单备注',
            property: 'remark',
            content: () => (
              <Input type='textarea' v-model={formModel.remark} rows={3} maxlength={255} resize={false}></Input>
            ),
          },
        ],
      },
    ]);
    const unit = ref('month' as 'month' | 'year');
    const priceTableData = [
      {
        billingItem: '实例费用',
        billingMode: '包年包月',
        price: '114.00 元',
      },
      {
        billingItem: '网络费用',
        billingMode: '包月',
        price: '12.02 元',
      },
    ];

    // 绑定弹性IP弹窗
    const isBindEipDialogShow = ref(false);
    const selectedEipData = reactive({ id: '', name: '', elasticIP: '' });
    const bindEipTable = useTable({
      columns: [
        {
          label: 'ID',
          field: 'id',
          render: ({ data }: any) => {
            return <Radio v-model={selectedEipData.id} label={data.id} />;
          },
        },
        {
          label: '名称',
          field: 'name',
        },
        {
          label: '弹性公网IP',
          field: 'elasticIP',
        },
      ],
      searchData: [
        {
          name: 'ID',
          id: 'id',
        },
        {
          name: '名称',
          id: 'name',
        },
        {
          name: '弹性公网IP',
          id: 'elasticIP',
        },
      ],
      searchUrl: '',
      tableData: [
        {
          id: '1',
          name: '服务器A',
          elasticIP: '123.123.123.123',
        },
        {
          id: '2',
          name: '服务器B',
          elasticIP: '234.234.234.234',
        },
        {
          id: '3',
          name: '服务器C',
          elasticIP: '345.345.345.345',
        },
      ],
      noSearch: true,
      tableExtraOptions: {
        onRowClick: (_event: Event, _row: any) => {
          Object.assign(selectedEipData, _row);
        },
      },
    });
    const handleBindEip = () => {
      formModel.eip_id = selectedEipData.id;
    };

    // 性能容量型弹窗
    const isClbSpecTypeDialogShow = ref(false);
    const selectedClbSpecType = reactive({
      model: '-1',
      maxConcurrentConnections: '',
      newConnectionsPerSecond: '',
      queriesPerSecond: '',
      bandwidthLimit: '',
    });
    const clbSpecTypeTable = useTable({
      columns: [
        {
          label: '型号',
          field: 'model',
          render: ({ data }: any) => {
            return <Radio v-model={selectedClbSpecType.model} label={data.model} />;
          },
        },
        {
          label: '最大并发连接数(个)',
          field: 'maxConcurrentConnections',
        },
        {
          label: '每秒新建连接数(个)',
          field: 'newConnectionsPerSecond',
        },
        {
          label: '每秒查询数(个)',
          field: 'queriesPerSecond',
        },
        {
          label: '带宽上限(Mbps)',
          field: 'bandwidthLimit',
        },
      ],
      searchData: [
        {
          name: '型号',
          id: 'model',
        },
        {
          name: '最大并发连接数(个)',
          id: 'maxConcurrentConnections',
        },
        {
          name: '每秒新建连接数(个)',
          id: 'newConnectionsPerSecond',
        },
        {
          name: '每秒查询数(个)',
          id: 'queriesPerSecond',
        },
        {
          name: '带宽上限(Mbps)',
          id: 'bandwidthLimit',
        },
      ],
      searchUrl: '',
      tableData: [
        {
          model: 'Model-A',
          maxConcurrentConnections: '10000',
          newConnectionsPerSecond: '500',
          queriesPerSecond: '2000',
          bandwidthLimit: '100',
        },
        {
          model: 'Model-B',
          maxConcurrentConnections: '20000',
          newConnectionsPerSecond: '1000',
          queriesPerSecond: '4000',
          bandwidthLimit: '200',
        },
        {
          model: 'Model-C',
          maxConcurrentConnections: '30000',
          newConnectionsPerSecond: '1500',
          queriesPerSecond: '6000',
          bandwidthLimit: '300',
        },
      ],
      tableExtraOptions: {
        onRowClick: (_event: Event, _row: any) => {
          Object.assign(selectedClbSpecType, _row);
        },
      },
    });
    const handleSelectClbSpecType = () => {
      formModel.clbSpecType = selectedClbSpecType.model;
    };

    watch(
      () => [formModel.netType, formModel.ipType],
      ([netType, ipType]) => {
        if (netType === 'intra' || ipType === 'ipv6' || ipType === 'ipv6_nat64') {
          formModel.instanceChargeType = '按量计费';
        } else {
          formModel.instanceChargeType = '包年包月';
        }
      },
    );

    watch(
      () => formModel.zoneType,
      (val) => {
        if (val === 'single') {
          formModel.zone = '';
        }
      },
    );

    watch(
      () => formModel.clbSpecType,
      (val) => {
        isClbSpecTypeDialogShow.value = !!val;
      },
    );

    watch(unit, () => {
      formModel.duration = 1;
    });

    return () => (
      <div class='apply-clb-page'>
        <DetailHeader>
          <p class='apply-clb-header-title'>购买负载均衡</p>
        </DetailHeader>
        <Form class='apply-clb-form-container' formType='vertical' model={formModel}>
          <ConditionOptions
            type={ResourceTypeEnum.CLB}
            v-model:bizId={formModel.bizId}
            v-model:cloudAccountId={formModel.cloudAccountId}
            v-model:vendor={formModel.vendor}
            v-model:region={formModel.region}
          />
          {formItemOptions.value.map(({ id, title, children }) => (
            <CommonCard key={id} title={() => title} class='form-card-container'>
              {children.map((item) => {
                let contentVNode = null;
                if (Array.isArray(item)) {
                  contentVNode = (
                    <div class='flex-row'>
                      {item.map(({ label, required, property, content, hidden }) => {
                        if (hidden) return null;
                        return (
                          <FormItem key={property} label={label} required={required} property={property}>
                            {content()}
                          </FormItem>
                        );
                      })}
                    </div>
                  );
                } else if (item.simpleShow) {
                  contentVNode = item.content();
                } else {
                  if (item.hidden) {
                    contentVNode = null;
                  } else {
                    contentVNode = (
                      <FormItem
                        key={item.property}
                        label={item.label}
                        required={item.required}
                        property={item.property}>
                        {item.content()}
                      </FormItem>
                    );
                  }
                }
                return contentVNode;
              })}
            </CommonCard>
          ))}
        </Form>
        <div class='apply-clb-bottom-bar'>
          <div class='info-wrap'>
            <span class='label'>IP资源费用</span>:
            <span class='value'>
              <span class='number'>0.01</span>
              <span class='unit'>元/小时</span>
            </span>
          </div>
          <div class='info-wrap'>
            <Popover theme='light' trigger='click' width={362} placement='top' offset={12}>
              {{
                default: () => <span class='label has-tips'>配置费用</span>,
                content: () => (
                  <Table data={priceTableData}>
                    <Column field='billingItem' label='计费项'></Column>
                    <Column field='billingMode' label='计费模式'></Column>
                    <Column field='price' label='价格' align='right'></Column>
                  </Table>
                ),
              }}
            </Popover>
            :
            <span class='value'>
              <span class='unit'>￥</span>
              <span class='number'>126.02</span>
            </span>
          </div>
          <div class='operation-btn-wrap'>
            <Button theme='primary'>立即购买</Button>
            <Button>取消</Button>
          </div>
        </div>
        <VpcPreviewDialog
          isShow={isVpcPreviewDialogShow.value}
          data={vpcData.value}
          handleClose={() => (isVpcPreviewDialogShow.value = false)}
        />
        <SubnetPreviewDialog
          isShow={isSubnetPreviewDialogShow.value}
          data={subnetData.value}
          handleClose={() => (isSubnetPreviewDialogShow.value = false)}
        />
        <CommonDialog
          v-model:isShow={isBindEipDialogShow.value}
          title='绑定弹性 IP'
          width={620}
          onHandleConfirm={handleBindEip}>
          <div>选择 EIP</div>
          <bindEipTable.CommonTable />
        </CommonDialog>
        <CommonDialog
          v-model:isShow={isClbSpecTypeDialogShow.value}
          title='选择实例规格'
          width={'60vw'}
          onHandleConfirm={handleSelectClbSpecType}>
          <clbSpecTypeTable.CommonTable />
        </CommonDialog>
      </div>
    );
  },
});
