<template>
  <div class="flex min-w-0 flex-col gap-2">
    <base-progress
        v-if="isIndeterminate"
        :id="`${group.id}-progress`"
        variant="warning"
        :value="0"
        :max="100"
    >
      {{ t('media.steps.paused.indeterminate') }}
    </base-progress>
    <base-progress
        v-else
        :id="`${group.id}-progress`"
        :max="100"
        :value="percent"
        variant="warning"
    >
      {{
        capitalizeFirstLetter(t('media.steps.paused.progress', {
          percentage: percent.toFixed(2),
        }))
      }}
    </base-progress>

    <div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 text-sm text-base-content/70">
      <span v-if="!isIndeterminate" class="tabular-nums">{{ t('media.steps.paused.metadata.eta', { eta: etaDisplay }) }}</span>
      <button type="button" class="btn btn-primary btn-sm ml-auto gap-1.5" @click="emit('resume')">
        <play-icon class="size-4" aria-hidden="true" />
        {{ t('common.resume') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, PropType, ref, watch } from 'vue';
import { PlayIcon } from '@heroicons/vue/24/solid';
import { useMediaProgressStore } from '../../../stores/media/progress';
import BaseProgress from '../../base/BaseProgress.vue';
import { capitalizeFirstLetter } from '../../../helpers/progress';
import { formatDuration } from '../../../helpers/units';
import { ProgressStage } from '../../../tauri/types/progress';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';

const i18n = useI18n();
const t = i18n.t;

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
const progress = computed(() => store.findDownloadProgress(group.id));

const isIndeterminate = computed(() => {
  if (!progress.value) return true;
  return progress.value.stage !== ProgressStage.downloading;
});

const percent = computed(() =>
  progress.value?.percentage ?? 0,
);

const rawEta = computed(() => progress.value?.etaSecs ?? 0);
const displayEta = ref(rawEta.value);

const alpha = 0.2;

watch(rawEta, (newEta) => {
  displayEta.value = Math.round(
    displayEta.value * (1 - alpha) + newEta * alpha,
  );
});

const etaDisplay = computed(() =>
  progress.value?.etaSecs != null ? formatDuration(progress.value.etaSecs) : '-',
);
</script>
