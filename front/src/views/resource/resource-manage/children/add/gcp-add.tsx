import { defineComponent, reactive, ref, watch } from 'vue';
import { Form, Input, Select, Radio, Button, Dialog, TagInput } from 'bkui-vue';
import { useI18n } from 'vue-i18n';
import { GCP_TYPE_STATUS, GCP_MATCH_STATUS, GCP_SOURCE_LIST, GCP_TARGET_LIST, GCP_PROTOCOL_LIST, GCP_EXECUTION_STATUS } from '@/constants';
import './gcp-add.scss';
export default defineComponent({
  name: 'GcpAdd',
  props: {
    isAdd: {
      type: Boolean,
    },
    detail: Object,
    isShow: {
      type: Boolean,
    },
    gcpTitle: {
      type: String,
    },
  },
  emits: ['update:isShow', 'submit'],
  setup(props, ctx) {
    const { t } = useI18n();
    const { FormItem } = Form;
    const { Option } = Select;
    const { Group } = Radio;
    const check = (val: any): boolean => {
      return  /^[a-z][a-z-z0-9_-]*$/.test(val);
    };
    const formRef = ref<InstanceType<typeof Form>>(null);
    // eslint-disable-next-line @typescript-eslint/prefer-optional-chain
    // const gcpPorts = computed(() => (state.projectModel[state.operate]
    //   && state.projectModel[state.operate]      // 端口
    //     .find((e: any) => e.protocol === state.protocol)?.ports));
    // // const ports = computed(() => (state.projectModel[state.operate]
    // //   && state.projectModel[state.operate]      // 端口
    // //     .find((e: any) => e.protocol === state.protocol)));
    // // console.log('ports', ports);
    const gcpPorts = ref([443]);
    const state = reactive({
      projectModel: {
        id: 0,
        type: 'egress',   // 账号类型
        name: 'test', // 名称
        priority: '', // 优先级
        vpc_id: '--',      // vpcid
        target_tags: [],
        destination_ranges: [],
        target_service_accounts: [],
        source_tags: [],
        source_service_accounts: [],
        source_ranges: [],
        bk_biz_id: '',      // 业务id
        create_at: '--',
        update_at: '--',
        disabled: false,
        allowed: [{
          protocol: 'tcp',
          ports: [
            '443',
          ],
        }],
        memo: '',
      },
      operate: 'allowed',
      target: 'target_tags',
      source: 'source_tags',
      protocol: 'tcp',
      formList: [
        {
          label: t('名称'),
          property: 'name',
          component: () => (
            <section class="w450">
                {props.isAdd ? (<Input class="w450" placeholder={t('请输入名称')} v-model={state.projectModel.name} />)
                  : (<span>{state.projectModel.name}</span>)}
            </section>
          ),
        },
        {
          label: t('业务'),
          property: 'resource-id',
          component: () => <span>{state.projectModel.bk_biz_id}</span>,
        },
        // {
        //   label: t('云厂商'),
        //   property: 'resource-id',
        //   component: () => <span>{state.projectModel.type}</span>,
        // },
        // {
        //   label: t('日志'),
        //   property: 'resource-id',
        //   component: () => <span>{state.projectModel.type}</span>,
        // },
        {
          label: 'VPC',
          property: 'vpc_id',
          component: () => <span>{state.projectModel.vpc_id}</span>,
        },
        {
          label: t('优先级'),
          property: 'priority',
          component: () => <Input class="w450" type='number' min={0} max={65535} placeholder={t('请输入优先级')} v-model={state.projectModel.priority} />,
        },
        {
          label: t('方向'),
          property: 'type',
          component: () => (
            <Group v-model={state.projectModel.type}>
                {GCP_TYPE_STATUS.map(e => (
                    <Radio label={e.value}>{t(e.label)}</Radio>
                ))}
            </Group>
          ),
        },
        {
          label: t('对匹配项执行的操作'),
          property: 'resource-id',
          component: () => (
            <Group v-model={state.operate}>
                {GCP_MATCH_STATUS.map(e => (
                    <Radio label={e.value}>{t(e.label)}</Radio>
                ))}
            </Group>
          ),
        },
        {
          label: t('目标'),
          property: 'target_tags',
          component: () => (
            <section class="flex-row">
                <Select v-model={state.target}>
                {GCP_TARGET_LIST.map(item => (
                    <Option
                        key={item.id}
                        value={item.id}
                        label={item.name}
                    >
                        {item.name}
                    </Option>
                ))
                }
                </Select>
                <TagInput class="w450 ml20" allow-create allow-auto-match placeholder={t('请输入目标')} list={[]} v-model={state.projectModel[state.target]} />
            </section>
          ),
        },
        {
          label: t('来源过滤条件'),
          property: 'name',
          component: () => (
            <section class="flex-row">
                <Select v-model={state.source}>
                {GCP_SOURCE_LIST.map(item => (
                    <Option
                        key={item.id}
                        value={item.id}
                        label={item.name}
                    >
                        {item.name}
                    </Option>
                ))
                }
                </Select>
                <TagInput class="w450 ml20" allow-create allow-auto-match placeholder={t('请输入过滤条件')} list={[]} v-model={state.projectModel[state.source]} />
            </section>
          ),
        },
        // {
        //   label: t('次要来源过滤条件'),
        //   property: 'name',
        //   component: () => (
        //     <section class="flex-row">
        //         <Select v-model={state.projectModel.name}>
        //         {GCP_SOURCE_LIST.map(item => (
        //             <Option
        //                 key={item.id}
        //                 value={item.id}
        //                 label={item.name}
        //             >
        //                 {item.name}
        //             </Option>
        //         ))
        //         }
        //         </Select>
        //         <Input class="w450 ml20" placeholder={t('请输入名称')} v-model={state.projectModel.name} />
        //     </section>
        //   ),
        // },
        {
          label: t('协议和端口'),
          property: 'name',
          component: () => (
            <section class="flex-row">
                <Select v-model={state.protocol}>
                {GCP_PROTOCOL_LIST.map(item => (
                    <Option
                        key={item.id}
                        value={item.id}
                        label={item.name}
                    >
                        {item.name}
                    </Option>
                ))
                }
                </Select>
                <TagInput class="w450 ml20" allow-create allow-auto-match list={[]} placeholder={t('请输入端口')} v-model={gcpPorts.value} onBlur={handleBlur} />
            </section>
          ),
        },
        {
          label: t('强制执行'),
          property: 'disabled',
          component: () => <Group v-model={state.projectModel.disabled}>
          {GCP_EXECUTION_STATUS.map(e => (
              <Radio label={e.value}>{t(e.label)}</Radio>
          ))}
          </Group>,
        },
        {
          label: t('创建时间'),
          property: 'resource-id',
          component: () => <span>{state.projectModel.create_at}</span>,
        },
        {
          label: t('修改时间'),
          property: 'resource-id',
          component: () => <span>{state.projectModel.update_at}</span>,
        },
        {
          label: t('备注'),
          property: 'memo',
          component: () => <Input class="w450" placeholder={t('请输入备注')} type="textarea" v-model={state.projectModel.memo} />,
        },
      ],
      formRules: {
        name: [
          { trigger: 'blur', message: '名称必须以小写字母开头，后面最多可跟 32个小写字母、数字或连字符，但不能以连字符结尾业务与项目至少填一个', validator: check },
        ],
      },
    });

    watch(() => props.isShow, (val) => {
      if (val) {
        console.log('detail', props.detail);
        // @ts-ignore
        state.projectModel = { ...props.detail };
      }
    });

    watch(() => state.target, (newValue, oldValue) => {
      if (newValue !== oldValue) {
        state.projectModel[oldValue] = [];
      }
    });

    watch(() => state.source, (newValue, oldValue) => {
      if (newValue !== oldValue) {
        state.projectModel[oldValue] = [];
      }
    });

    watch(() => state.operate, (newValue, oldValue) => {
      state.projectModel[newValue] = state.projectModel[oldValue];
      state.projectModel[oldValue] = [];
    });

    watch(() => state.protocol, () => {
      gcpPorts.value = state.projectModel[state.operate].find((e: any) => e.protocol === state.protocol)?.ports || [];
    });

    const handleBlur = () => {
      const protocolData = state.projectModel[state.operate].map((p: any) => p.protocol);
      if (!protocolData.includes(state.protocol)) {
        if (gcpPorts.value.length) {
          state.projectModel[state.operate].push({
            protocol: state.protocol,
            ports: gcpPorts.value,
          });
        }
      } else {
        state.projectModel[state.operate].forEach((e: any, index: number) => {
          if (e.protocol === state.protocol) {
            if (gcpPorts.value.length === 0) {
              state.projectModel[state.operate].splice(index, 1);
            } else {
              e.ports = gcpPorts.value;
            }
          }
        });
      }
    };


    const submit = () => {
      console.log('state.projectModel', state.projectModel);
      ctx.emit('submit', state.projectModel);
    };
    const cancel = () => {
      ctx.emit('update:isShow', false);
    };

    return () => (
        <Dialog
            isShow={props.isShow}
            title={props.gcpTitle}
            size="large"
            dialog-type="show">
            <Form model={state.projectModel} labelWidth={140} rules={state.formRules} ref={formRef} class="gcp-form">
                {state.formList.map(item => (
                    <FormItem label={item.label} property={item.property}>
                    {item.component()}
                    </FormItem>
                ))}
                <footer class="gcp-footer">
                    <Button class="w90" theme="primary" onClick={submit}>{t('确认')}</Button>
                    <Button class="w90 ml20" onClick={cancel}>{t('取消')}</Button>
                </footer>
            </Form>
        </Dialog>
    );
  },
});