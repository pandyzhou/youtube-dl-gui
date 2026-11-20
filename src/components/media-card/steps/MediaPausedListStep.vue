<template>
  <div class="flex min-w-0 flex-col gap-2">
    <base-progress
        :id="`${group.id}-progress`"
        :max="100"
        :value="percent"
        variant="warning"
    >
      {{
        t('media.steps.paused.progressList', {
          percentage: percent.toFixed(2),
          done: progress?.done ?? 0,
          total: progress?.total ?? 0,
        })
      }}
    </base-progress>

    <div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 text-sm text-base-content/70">
      <span>{{ t('media.steps.paused.metadata.etaItems', { n: remainingItems }) }}</span>
      <button type="button" class="btn btn-primary btn-sm ml-auto gap-1.5" @click="emit('resume')">
        <play-icon class="size-4" aria-hidden="true" />
        {{ t('common.resume') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue';
import { PlayIcon } from '@heroicons/vue/24/solid';
import { useMediaProgressStore } from '../../../stores/media/progress';
import BaseProgress from '../../base/BaseProgress.vue';
import { computeGroupPercentage } from '../../../helpers/progress';
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
  (e: 'resume'): void;
}>();

const store = useMediaProgressStore();
const progress = computed(() => store.findGroupProgress(group.id));

const percent = computed(() => {
  return computeGroupPercentage(progress.value);
});

const remainingItems = computed(() => {
  return Math.max(0, (progress.value?.total ?? 0) - (progress.value?.done ?? 0));
});
</script>
