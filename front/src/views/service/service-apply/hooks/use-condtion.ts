import { VendorEnum } from '@/common/constant';
import { Senarios, useWhereAmI } from '@/hooks/useWhereAmI';
import { computed, reactive } from 'vue';

export type Cond = {
  bizId: number;
  cloudAccountId: string;
  vendor: Lowercase<keyof typeof VendorEnum> | string;
  region: string;
  resourceGroup?: string;
};

export default () => {
  const { getBizsId, whereAmI } = useWhereAmI();

  const cond = reactive<Cond>({
    bizId: whereAmI.value === Senarios.business ? getBizsId() : 0,
    cloudAccountId: '',
    vendor: '',
    region: '',
    resourceGroup: '',
  });

  const isEmptyCond = computed(() => {
    const isResourcePage = whereAmI.value === Senarios.resource;
    const isEmpty = !cond.cloudAccountId || !cond.vendor || !cond.region || (!isResourcePage && !cond.bizId);
    if (cond.vendor === VendorEnum.AZURE) {
      return isEmpty || !cond.resourceGroup;
    }
    return isEmpty;
  });

  return {
    cond,
    isEmptyCond,
  };
};
