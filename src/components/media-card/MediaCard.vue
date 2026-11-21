<template>
  <article
      class="card min-w-0 flex-row gap-4 border bg-base-100 p-3 transition-colors duration-150"
      :class="statusBorder"
  >
    <figure class="relative flex aspect-video w-40 shrink-0 items-center justify-center self-start overflow-hidden rounded-lg bg-base-300">
      <img
          v-if="showThumbnail"
          :src="group.thumbnail"
          :alt="t('media.view.thumbnailAlt', { title: group.title ?? '' })"
          class="h-full w-full object-cover"
          loading="lazy"
          @error="thumbnailFailed = true"
      />
      <film-icon v-else class="size-8 text-base-content/25" aria-hidden="true" />
    </figure>
    <div class="flex min-w-0 flex-1 flex-col gap-2">
      <div class="flex min-w-0 items-start gap-2">
        <h2 :title="title" class="min-w-0 flex-1 truncate text-[15px] leading-8 font-semibold">{{ title }}</h2>
        <media-card-actions :group="group" @remove="removeGroup" />
      </div>
      <component
          :is="activeStep"
          v-if="activeStep"
          v-bind="activeStepProps"
          @download="downloadGroup"
          @pause="pauseGroup"
          @resume="downloadGroup"
          @retry="retryGroup"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ComputedRef, PropType, ref, watch } from 'vue';
import { FilmIcon } from '@heroicons/vue/24/outline';
import { MediaState, useMediaStateStore } from '../../stores/media/state';
import { useMediaStore } from '../../stores/media/media';
import { useMediaOptionsStore } from '../../stores/media/options';
import FetchStep from './steps/FetchStep.vue';
import FetchListStep from './steps/FetchListStep.vue';
import MediaConfigureStep from './steps/MediaConfigureStep.vue';
import MediaDownloadStep from './steps/MediaDownloadStep.vue';
import MediaDoneStep from './steps/MediaDoneStep.vue';
import MediaErrorStep from './steps/MediaErrorStep.vue';
import MediaCardActions from './actions/MediaCardActions.vue';
import MediaDownloadListStep from './steps/MediaDownloadListStep.vue';
import PlaylistSelectionStep from './steps/PlaylistSelectionStep.vue';
import { useToastStore } from '../../stores/toast';
import { Group } from '../../tauri/types/group';
import { useI18n } from 'vue-i18n';
import MediaPausedStep from './steps/MediaPausedStep.vue';
import MediaPausedListStep from './steps/MediaPausedListStep.vue';

const stepMap = {
  [MediaState.fetching]: FetchStep,
  [MediaState.fetchingList]: FetchListStep,
  [MediaState.playlistSelection]: PlaylistSelectionStep,
  [MediaState.configure]: MediaConfigureStep,
  [MediaState.downloading]: MediaDownloadStep,
  [MediaState.downloadingList]: MediaDownloadListStep,
  [MediaState.paused]: MediaPausedStep,
  [MediaState.pausedList]: MediaPausedListStep,
  [MediaState.done]: MediaDoneStep,
  [MediaState.error]: MediaErrorStep,
} as const;

const stateStore = useMediaStateStore();
const mediaStore = useMediaStore();
const toastStore = useToastStore();
const optionsStore = useMediaOptionsStore();

const { t } = useI18n();

const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

const groupState: ComputedRef<MediaState | undefined> = computed(() => stateStore.getGroupState(group.id));

const activeStep = computed(() => stepMap[groupState.value ?? MediaState.fetching]);

const activeStepProps = computed(() => ({ group }));

const title = computed(() => group.title ?? group.url);

const downloadGroup = (): void => {
  const options = optionsStore.getOptions(group.id);
  if (!options) {
    // TODO show a toast?
    console.warn(`No options found for group: ${group.id}, cannot download.`);
    return;
  }
  void mediaStore.downloadGroup(group.id, options);
};

const pauseGroup = (): void => {
  void mediaStore.pauseGroup(group.id);
};

const retryGroup = (): void => {
  if (!group.url) {
    toastStore.showToast(t('media.card.toasts.retryError'), { style: 'error' });
    return;
  }
  mediaStore.dispatchMediaInfoFetch(group.url)
    .then(() => {
      toastStore.showToast(t('media.card.toasts.retry'));
    })
    .catch(() => {
      toastStore.showToast(t('media.card.toasts.retryError'), { style: 'error' });
    });
};

const removeGroup = (): void => {
  mediaStore.deleteGroup(group?.id);
};

// No thumbnail yet (still fetching), none at all, or it failed to load: show a
// quiet icon instead of a placeholder picture.
const thumbnailFailed = ref(false);
watch(() => group.thumbnail, () => {
  thumbnailFailed.value = false;
});
const showThumbnail = computed(() => Boolean(group.thumbnail) && !thumbnailFailed.value);

const statusBorder = computed(() => {
  switch (groupState.value) {
    case MediaState.error:
      return 'border-error/50';
    case MediaState.paused:
    case MediaState.pausedList:
      return 'border-warning/50';
    default:
      return 'border-line';
  }
});

</script>
