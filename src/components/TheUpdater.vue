<template>
  <Transition
    appear
    enter-active-class="transition duration-250 ease-out motion-reduce:transition-none"
    enter-from-class="translate-y-2 opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in motion-reduce:transition-none"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-2 opacity-0"
  >
    <div
      v-if="!isIgnored && checkResult?.available"
      role="alert"
      data-testid="updater-alert"
      class="mb-3 flex flex-col gap-2 rounded-box border border-info/30 bg-info/10 p-3 text-sm"
    >
      <div class="flex items-start gap-2">
        <arrow-path-icon
          class="mt-0.5 size-4 shrink-0 text-info"
          :class="{ 'animate-spin motion-reduce:animate-none': isUpdating }"
          aria-hidden="true"
        />
        <h3 class="min-w-0 font-semibold leading-5">{{ title }}</h3>
      </div>

      <template v-if="!isUpdating && !isNeedingRestart">
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-xs flex-1 px-2" type="button" @click="updaterStore.ignore()">
            {{ t('common.later') }}
          </button>
          <button class="btn btn-info btn-xs flex-1 px-2" type="button" @click="updaterStore.download()">
            {{ t('common.download') }}
          </button>
        </div>
      </template>
      <template v-else-if="isNeedingRestart">
        <p class="text-xs leading-5 text-base-content/70">{{ t('updater.install.subtitle') }}</p>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-xs flex-1 px-2" type="button" @click="updaterStore.ignore()">
            {{ t('common.later') }}
          </button>
          <button class="btn btn-info btn-xs flex-1 px-2" type="button" @click="updaterStore.install()">
            {{ t('common.install') }}
          </button>
        </div>
      </template>
      <div v-else class="flex items-center gap-2">
        <progress class="progress progress-info h-1.5 flex-1" :value="progress" max="100"></progress>
        <p class="text-xs tabular-nums">{{ t('common.percentage', { percent: progress.toFixed() }) }}</p>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ArrowPathIcon } from '@heroicons/vue/24/solid';
import { useUpdaterStore } from '../stores/updater';
import { computed } from 'vue';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const updaterStore = useUpdaterStore();
const { checkResult, downloadProgress, isIgnored, isUpdating, isNeedingRestart } = storeToRefs(updaterStore);

const title = computed(() => {
  if (isNeedingRestart.value) return t('updater.install.title');
  if (isUpdating.value) return t('updater.downloading');
  return t('updater.available');
});

const progress = computed(() => {
  const received = downloadProgress.value.received ?? 0;
  const total = downloadProgress.value.total ?? 0;
  if (total === 0 && received === 0) {
    return 0;
  }
  return received / total * 100;
});
</script>
