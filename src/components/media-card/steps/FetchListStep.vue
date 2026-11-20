<template>
  <div class="flex min-w-0 flex-col gap-2">
    <base-progress
        :id="`${group.id}-progress`"
        :max="group?.total"
        :value="group?.processed"
    >
      {{ t('media.steps.fetch.progress', { percentage, done: group?.processed ?? '0', total: group?.total ?? '1' }) }}
    </base-progress>
    <p class="text-sm text-base-content/60">{{ t('media.steps.fetch.progressLabel') }}</p>
  </div>
</template>

<script setup lang="ts">

import { computed, PropType } from 'vue';
import BaseProgress from '../../base/BaseProgress.vue';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

const percentage = computed((): string => {
  const rawPercentage = group.processed / group.total * 100;
  return rawPercentage.toFixed(2);
});

</script>
