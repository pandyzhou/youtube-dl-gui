<template>
  <section class="mx-auto flex w-full max-w-4xl flex-col gap-5 px-6 pt-6 md:flex-row">
    <figure class="aspect-video w-full shrink-0 overflow-hidden rounded-box bg-base-300 md:w-80">
      <img
        :src="group?.thumbnail ?? placeholderUrl"
        :alt="t('media.view.thumbnailAlt', { title: group?.title ?? group?.url ?? t('common.video') })"
        class="h-full w-full object-cover"
        @error="setPlaceholderImage"
      />
    </figure>
    <div class="flex min-w-0 flex-1 flex-col gap-3">
      <h1 class="text-xl font-semibold break-words">{{ group?.title ?? group?.url }}</h1>
      <dl v-if="metadataItems.length" class="flex flex-col divide-y divide-line rounded-box border border-line bg-base-100 text-sm">
        <div v-for="item in metadataItems" :key="item.label" class="flex gap-3 px-4 py-2.5">
          <dt class="w-24 shrink-0 text-base-content/60">{{ item.label }}</dt>
          <dd class="min-w-0 flex-1">
            <a
              v-if="item.href"
              class="link link-primary break-all"
              target="_blank"
              rel="noopener"
              :href="item.href"
            >
              {{ item.value }}
            </a>
            <span v-else class="break-words">{{ item.value }}</span>
          </dd>
        </div>
      </dl>
    </div>
  </section>
  <section class="mx-auto flex w-full max-w-4xl flex-col gap-5 px-6 pt-5 pb-6">
    <div class="media-stats-scroll overflow-x-auto overflow-y-hidden">
      <base-media-stats v-if="group" :item="group" />
    </div>
    <div class="rounded-box border border-line bg-base-100 p-4" v-if="group?.description">
      <h2 class="mb-2 text-[15px] font-semibold">{{ t('media.view.metadata.description') }}</h2>
      <p class="max-h-56 overflow-y-auto text-sm leading-6 whitespace-pre-wrap text-base-content/80">
        {{ group.description }}
      </p>
    </div>
    <media-chapter-timeline
        v-if="group?.chapters?.length"
        :chapters="group.chapters"
        :duration="group.duration"
    />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import placeholderUrl from '../../assets/placeholder.png';
import BaseMediaStats from '../base/BaseMediaStats.vue';
import MediaChapterTimeline from './MediaChapterTimeline.vue';
import { useI18n } from 'vue-i18n';
import { useMediaGroupStore } from '../../stores/media/group.ts';

const { t } = useI18n();
const groupStore = useMediaGroupStore();

const props = defineProps<{
  groupId: string;
}>();

const group = computed(() => groupStore.findGroupById(props.groupId));

type MetadataItem = {
  label: string;
  value: string;
  href?: string;
};

const metadataItems = computed<MetadataItem[]>(() => {
  const currentGroup = group.value;

  if (!currentGroup) {
    return [];
  }

  return [
    currentGroup.uploader
      ? { label: t('media.view.metadata.uploader'), value: currentGroup.uploader }
      : undefined,
    currentGroup.extractor
      ? { label: t('media.view.metadata.extractor'), value: currentGroup.extractor }
      : undefined,
    currentGroup.url
      ? { label: t('media.view.metadata.link'), value: currentGroup.url, href: currentGroup.url }
      : undefined,
  ].filter((item): item is MetadataItem => Boolean(item));
});

const setPlaceholderImage = (event: Event): void => {
  if (event.target instanceof HTMLImageElement) {
    event.target.src = placeholderUrl;
  }
};
</script>

<style scoped>
.media-stats-scroll {
  scrollbar-color: color-mix(in oklab, var(--color-base-content) 28%, transparent) transparent;
}

.media-stats-scroll::-webkit-scrollbar {
  height: 10px;
}

.media-stats-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.media-stats-scroll::-webkit-scrollbar-thumb {
  background: color-mix(in oklab, var(--color-base-content) 28%, transparent);
  border-radius: 9999px;
}

.media-stats-scroll::-webkit-scrollbar-thumb:hover {
  background: color-mix(in oklab, var(--color-base-content) 40%, transparent);
}
</style>
