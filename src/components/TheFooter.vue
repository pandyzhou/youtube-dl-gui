<template>
  <footer class="border-t border-line bg-base-100 px-6 py-3">
    <div class="flex flex-wrap items-center gap-x-5 gap-y-3">
      <div class="flex min-w-52 flex-1 flex-col gap-1.5">
        <p class="truncate text-sm" :class="isCompleted ? 'text-success' : ''" aria-live="polite">
          <template v-if="isDownloading">
            {{ t('layout.footer.progress.downloading', { done: progress.done, total: progress.total }) }}
          </template>
          <template v-else-if="isCompleted">
            {{ t('layout.footer.progress.completed', progress.total) }}
          </template>
          <template v-else-if="hasGroups">
            {{ t('layout.footer.progress.ready', progress.ready) }}
          </template>
          <template v-else>
            {{ t('layout.footer.progress.empty') }}
          </template>
        </p>
        <progress
            v-if="isDownloading || isCompleted"
            class="progress h-1.5 w-full"
            :class="isCompleted ? 'progress-success' : 'progress-primary'"
            :max="progress.total"
            :value="progress.done"
        />
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm whitespace-nowrap text-base-content/60" aria-hidden="true">
          {{ t('layout.footer.format.label') }}
        </span>
        <media-download-options
            v-model="selectedOptions"
            :formats="groupStore.getAllFormats()"
            :auto-select="false"
            locale-key="layout.footer.format"
            class="w-fit"
            join
        />
        <base-button-dropdown
            placement="top"
            align="end"
            btnClass="btn-soft"
            :disabled="!hasGroups"
            :caret-disabled="!hasQueueActions"
            :mainTooltip="t('layout.footer.queue.empty')"
            :caretAriaLabel="t('layout.footer.queue.more')"
            @mainClick="clearGroups"
        >
          <template #main>
            <span class="sr-only">{{ t('layout.footer.queue.empty') }}</span>
            <trash-icon class="size-5" aria-hidden="true" />
          </template>

          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasSuccessfulGroups" @click="clearSuccessfulGroups">
              <broom-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.clearSuccessful') }}
            </button>
          </li>
          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasErroredGroups" @click="clearErroredGroups">
              <broom-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.clearErrored') }}
            </button>
          </li>
          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasPendingGroups" @click="clearPendingGroups">
              <broom-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.clearPending') }}
            </button>
          </li>
          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasDownloadingGroups" @click="cancelDownloadingGroups">
              <x-mark-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.cancelDownloading') }}
            </button>
          </li>
          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasDownloadingGroups" @click="pauseAllGroups">
              <pause-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.pause') }}
            </button>
          </li>
          <li>
            <button class="gap-2 text-nowrap disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!hasPausedGroups" @click="resumeAllGroups">
              <play-icon class="w-4 h-4" />
              {{ t('layout.footer.queue.resume') }}
            </button>
          </li>
        </base-button-dropdown>
        <base-button
            class="btn-primary gap-1.5 px-5 whitespace-nowrap"
            :disabled="progress.ready <= 0"
            :loading="isStartingDownload"
            @click="downloadAll"
        >
          <arrow-down-tray-icon class="size-5" aria-hidden="true" />
          {{ t('layout.footer.downloadAll') }}
        </base-button>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { PauseIcon, PlayIcon, XMarkIcon } from '@heroicons/vue/24/solid';
import { ArrowDownTrayIcon, TrashIcon } from '@heroicons/vue/24/outline';
import { computed, ref, watch } from 'vue';
import BaseButton from './base/BaseButton.vue';
import { useMediaStore } from '../stores/media/media';
import { useMediaProgressStore } from '../stores/media/progress';
import { DownloadOptions } from '../tauri/types/media';
import { useMediaOptionsStore } from '../stores/media/options';
import { useI18n } from 'vue-i18n';
import MediaDownloadOptions from './media-card/MediaDownloadOptions.vue';
import { useMediaGroupStore } from '../stores/media/group.ts';
import BaseButtonDropdown from './base/BaseButtonDropdown.vue';
import { MediaState, useMediaStateStore } from '../stores/media/state.ts';
import BroomIcon from './icons/BroomIcon.vue';

const i18n = useI18n();
const t = i18n.t;

const mediaStore = useMediaStore();
const mediaStateStore = useMediaStateStore();
const progressStore = useMediaProgressStore();
const groupStore = useMediaGroupStore();
const optionsStore = useMediaOptionsStore();

const isStartingDownload = ref(false);

const clearGroups = (): void => {
  mediaStore.deleteAllGroups();
};

const clearSuccessfulGroups = (): void => {
  mediaStore.deleteGroupsByState([MediaState.done]);
};

const clearErroredGroups = (): void => {
  mediaStore.deleteGroupsByState([MediaState.error]);
};

const clearPendingGroups = (): void => {
  mediaStore.deleteGroupsByState([
    MediaState.fetching,
    MediaState.fetchingList,
    MediaState.playlistSelection,
    MediaState.configure,
    MediaState.paused,
    MediaState.pausedList,
  ]);
};

const cancelDownloadingGroups = (): void => {
  mediaStore.deleteGroupsByState([MediaState.downloading, MediaState.downloadingList]);
};

const progress = computed(() => {
  return progressStore.findAllProgress();
});

const isDownloading = computed(() => {
  return (progress.value.downloading ?? 0) > 0;
});

const isCompleted = computed(() => {
  return (progress.value.downloading ?? 0) === 0 && (progress.value.ready ?? 0) === 0 && (progress.value.done ?? 0) > 0;
});

const selectedOptions = ref<DownloadOptions | undefined>(optionsStore.getGlobalOptions());

function applySelection() {
  const globalOptions = selectedOptions.value;
  if (!globalOptions) return;
  optionsStore.applyGlobalOptions(globalOptions);
}

watch(selectedOptions, applySelection, { immediate: true });

const downloadAll = async (): Promise<void> => {
  isStartingDownload.value = true;
  try {
    await mediaStore.downloadAllGroups();
  } catch (e) {
    console.error(e);
  } finally {
    isStartingDownload.value = false;
  }
};

const pauseAllGroups = (): void => {
  mediaStore.pauseAllGroups();
};

const resumeAllGroups = (): void => {
  mediaStore.resumeAllGroups();
};

const hasGroups = computed(() => groupStore.countGroups() > 0);
const hasSuccessfulGroups = computed(() => mediaStateStore.hasGroupWithState(MediaState.done));
const hasErroredGroups = computed(() => mediaStateStore.hasGroupWithState(MediaState.error));
const hasPendingGroups = computed(() => mediaStateStore.hasGroupWithState(
  MediaState.fetching,
  MediaState.fetchingList,
  MediaState.playlistSelection,
  MediaState.configure,
  MediaState.paused,
  MediaState.pausedList,
));
const hasPausedGroups = computed(() => mediaStateStore.hasGroupWithState(MediaState.paused, MediaState.pausedList));
const hasDownloadingGroups = computed(() => mediaStateStore.hasGroupWithState(MediaState.downloading, MediaState.downloadingList));
const hasQueueActions = computed(() => (
  hasSuccessfulGroups.value
  || hasErroredGroups.value
  || hasPendingGroups.value
  || hasPausedGroups.value
  || hasDownloadingGroups.value
));
</script>
