<template>
  <article>
    <base-sub-nav back-button>
      <button v-if="isPaused" @click="downloadItem" class="btn btn-warning gap-1.5"><play-icon class="size-4" aria-hidden="true" />{{ t('common.resume') }}</button>
      <button v-else @click="downloadItem" class="btn btn-primary gap-1.5"><arrow-down-tray-icon class="size-4" aria-hidden="true" />{{ t('common.download') }}</button>
      <template v-slot:title>
        <div role="tablist" class="seg-tabs">
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'preferences.quality', params: { groupId } }" class="seg-tab">{{ t('media.preferences.tabs.quality') }}</router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'preferences.output', params: { groupId } }" class="seg-tab">{{ t('settings.output.legend') }}</router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'preferences.subtitles', params: { groupId } }" class="seg-tab">{{ t('subtitles.title') }}</router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'preferences.network', params: { groupId } }" class="seg-tab">{{ t('media.preferences.tabs.network') }}</router-link>
        </div>
      </template>
    </base-sub-nav>
    <router-view/>
  </article>
</template>

<script setup lang="ts">
import { ArrowDownTrayIcon, PlayIcon } from '@heroicons/vue/24/outline';

import { useMediaStore } from '../../stores/media/media';
import { useRouter } from 'vue-router';
import BaseSubNav from '../../components/base/BaseSubNav.vue';
import { useI18n } from 'vue-i18n';
import { useMediaOptionsStore } from '../../stores/media/options.ts';
import { MediaState, useMediaStateStore } from '../../stores/media/state.ts';
import { computed } from 'vue';

const { groupId } = defineProps({
  groupId: {
    type: String,
    required: true,
  },
});

const { t } = useI18n();
const router = useRouter();
const mediaStore = useMediaStore();
const stateStore = useMediaStateStore();
const optionsStore = useMediaOptionsStore();

const isPaused = computed(() => {
  const state = stateStore.getGroupState(groupId);
  return state === MediaState.paused || state === MediaState.pausedList;
});

const downloadItem = (): void => {
  const options = optionsStore.getOptions(groupId);
  if (!options) {
    console.warn(`No options found for group: ${groupId}, cannot download.`);
    return;
  }
  void mediaStore.downloadGroup(groupId, options);
  void router.push(`/`);
};

</script>

<style scoped>
.tabs.tabs-box {
  background-color: var(--color-base-200);
  border-radius: var(--radius-box);
}

.tab {
  color: var(--color-base-content);
  border-radius: var(--radius-field);
  transition: background-color 0.15s ease, color 0.15s ease;
}

.tab:hover:not(.tab-active) {
  background-color: var(--color-neutral);
}

.tab-active {
  background-color: var(--color-neutral);
  border: var(--border) solid var(--color-primary);
}
</style>
