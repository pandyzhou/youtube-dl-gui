<template>
  <div class="flex min-w-0 flex-col gap-2">
    <base-progress
        :id="`${group.id}-progress`"
        :max="100"
        :value="percent"
    >
      {{
        t('media.steps.download.progressList', {
            percentage: percent.toFixed(2),
            done: progress?.done ?? 0,
            total: progress?.total ?? 0,
        })
      }}
    </base-progress>

    <div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 text-sm text-base-content/70">
      <span class="tabular-nums">{{ t('media.steps.download.metadata.speed', { speed: speedDisplay }) }}</span>
      <button type="button" class="btn btn-soft btn-sm ml-auto gap-1.5" @click="emit('pause')">
        <pause-icon class="size-4" aria-hidden="true" />
        {{ t('common.pause') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue';
import { PauseIcon } from '@heroicons/vue/24/solid';
import { useMediaProgressStore } from '../../../stores/media/progress';
import BaseProgress from '../../base/BaseProgress.vue';
import { computeGroupPercentage } from '../../../helpers/progress';
import { formatBytesPerSec } from '../../../helpers/units';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

const emit = defineEmits<{
  (e: 'pause'): void;
}>();

const store = useMediaProgressStore();
const progress = computed(() => store.findGroupProgress(group.id));

const percent = computed(() => {
  return computeGroupPercentage(progress.value);
});

const speedDisplay = computed(() =>
  progress.value?.speedBps != null
    ? formatBytesPerSec(progress.value.speedBps)
    : '-',
);
</script>
