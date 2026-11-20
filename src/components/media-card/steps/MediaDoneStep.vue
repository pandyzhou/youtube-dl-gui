<template>
  <div class="flex min-w-0 flex-col gap-2.5">
    <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
      <span class="badge badge-soft badge-success shrink-0 gap-1">
        <check-circle-icon class="size-4" aria-hidden="true" />
        {{ t('media.steps.done.badge') }}
      </span>
      <span v-if="singleSkippedReason" class="min-w-0 text-base-content/70">
        {{ singleSkippedReason }}
      </span>
      <span v-else-if="itemOutcomeDisplay" class="min-w-0 text-base-content/70">
        {{ t('media.steps.configure.metadata.items', { amount: group.total, details: itemOutcomeDisplay }) }}
      </span>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <div
          class="tooltip-bottom"
          :class="{ 'tooltip before:p-3': !groupDestination }"
          :data-tip="t('media.steps.done.undeterminedLocation')"
      >
        <button type="button" :disabled="!groupDestination" @click="openFile" class="btn btn-sm btn-primary gap-1.5">
          <play-circle-icon class="size-4" aria-hidden="true" />
          {{ group.isCombined ? t('media.steps.done.openFirst') : t('media.steps.done.open') }}
        </button>
      </div>
      <div
          class="tooltip-bottom"
          :class="{ 'tooltip before:p-3': !groupDestination }"
          :data-tip="t('media.steps.done.undeterminedLocation')"
      >
        <button type="button" :disabled="!groupDestination" @click="revealFile" class="btn btn-sm btn-soft gap-1.5">
          <folder-open-icon class="size-4" aria-hidden="true" />
          {{ t('media.steps.done.showFolder') }}
        </button>
      </div>
      <button type="button" class="btn btn-sm btn-ghost ml-auto gap-1.5 text-base-content/70" @click="emit('retry')">
        <arrow-path-icon class="size-4" aria-hidden="true" />
        {{ t('media.card.actions.retry') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">

import { computed, PropType } from 'vue';
import { ArrowPathIcon, CheckCircleIcon, FolderOpenIcon, PlayCircleIcon } from '@heroicons/vue/24/outline';
import { useOpener } from '../../../composables/useOpener';
import { useMediaDestinationStore } from '../../../stores/media/destination';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';
import { countSkippedDiagnostics, groupSkippedDiagnostics } from '../../../helpers/skippedDiagnostics.ts';
import { useMediaDiagnosticsStore } from '../../../stores/media/diagnostics.ts';

const { t } = useI18n();
const { openPath, revealPath } = useOpener();
const destinationStore = useMediaDestinationStore();
const diagnosticsStore = useMediaDiagnosticsStore();

const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

const emit = defineEmits<{
  (e: 'retry'): void;
}>();

const groupDestination = computed(() => {
  const destination = destinationStore.getPrimaryDestination(group.id);
  return destination?.path;
});

const skippedCount = computed(() => countSkippedDiagnostics(diagnosticsStore.findDiagnosticsByGroupId(group.id)));
const skippedGroups = computed(() => groupSkippedDiagnostics(diagnosticsStore.findDiagnosticsByGroupId(group.id)));

const singleSkippedReason = computed(() => {
  if (group.isCombined || group.errored > 0 || skippedGroups.value.length === 0) {
    return '';
  }

  return skippedGroups.value[0]?.message ?? '';
});

const itemOutcomeDisplay = computed(() => {
  if (group.errored > 0 && skippedCount.value > 0) {
    return t('media.steps.configure.metadata.failedAndSkippedCount', {
      failed: group.errored,
      skipped: skippedCount.value,
    });
  }

  if (group.errored > 0) {
    return t('media.steps.configure.metadata.failedCount', { amount: group.errored });
  }

  if (skippedCount.value > 0) {
    return t('media.steps.configure.metadata.skippedCount', { amount: skippedCount.value });
  }

  return '';
});

const openFile = async () => {
  const destination = groupDestination.value;
  if (!destination) return;
  await openPath(destination);
};

const revealFile = async () => {
  const destination = groupDestination.value;
  if (!destination) return;
  await revealPath(destination);
};

</script>
