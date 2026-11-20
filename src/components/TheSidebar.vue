<template>
  <nav
    :aria-label="t('layout.sidebar.label')"
    class="flex w-48 shrink-0 flex-col border-r border-line bg-base-300 px-3 pt-4 pb-3"
  >
    <div class="mb-5 flex items-center gap-2.5 px-2">
      <img src="/icon.png" alt="" class="size-8 shrink-0 rounded-lg" />
      <span class="min-w-0 text-sm leading-tight font-semibold">{{ appName }}</span>
    </div>

    <ul class="flex flex-col gap-0.5">
      <li v-for="item in mainItems" :key="item.key">
        <router-link
          :to="item.to"
          :class="itemClass(item.active)"
          :aria-current="item.active ? 'page' : undefined"
        >
          <component :is="item.icon" class="size-5 shrink-0" aria-hidden="true" />
          <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
          <span
            v-if="item.badge"
            class="min-w-6 rounded-full px-1.5 text-center text-xs leading-5 font-semibold tabular-nums"
            :class="item.badgeActive ? 'bg-primary text-primary-content' : 'bg-base-content/10'"
          >
            <span aria-hidden="true">{{ item.badge }}</span>
            <span class="sr-only">{{ t('layout.sidebar.queueCount', { n: item.badge }) }}</span>
          </span>
          <span v-else-if="item.on" class="size-2 shrink-0 rounded-full bg-primary">
            <span class="sr-only">{{ t('layout.sidebar.enabled') }}</span>
          </span>
        </router-link>
      </li>
    </ul>

    <div class="min-h-4 flex-1" />

    <the-updater />

    <router-link
      :to="{ name: 'settings.downloads' }"
      :class="itemClass(isSettings)"
      :aria-current="isSettings ? 'page' : undefined"
    >
      <cog-6-tooth-icon class="size-5 shrink-0" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate">{{ t('layout.sidebar.settings') }}</span>
    </router-link>
    <router-link
      v-if="appVersion"
      :to="{ name: 'settings.about' }"
      class="mt-2 truncate rounded px-2.5 text-xs text-base-content/50 transition-colors hover:text-base-content"
    >
      {{ t('layout.sidebar.version', { version: appVersion }) }}
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue';
import { useRoute, type RouteLocationRaw } from 'vue-router';
import { useI18n } from 'vue-i18n';
import {
  ChatBubbleBottomCenterTextIcon,
  Cog6ToothIcon,
  FolderIcon,
  FunnelIcon,
  KeyIcon,
  QueueListIcon,
} from '@heroicons/vue/24/outline';
import TheUpdater from './TheUpdater.vue';
import { useSettingsStore } from '../stores/settings';
import { useStrongholdStore } from '../stores/stronghold';
import { useMediaGroupStore } from '../stores/media/group';
import { MediaState, useMediaStateStore } from '../stores/media/state';
import { isInputFiltersActive } from '../helpers/inputFilters';

interface NavItem {
  key: string;
  to: RouteLocationRaw;
  icon: Component;
  label: string;
  active: boolean;
  badge?: number;
  badgeActive?: boolean;
  on?: boolean;
}

const { t } = useI18n();
const route = useRoute();
const settingsStore = useSettingsStore();
const strongholdStore = useStrongholdStore();
const groupStore = useMediaGroupStore();
const stateStore = useMediaStateStore();

const appName = 'Open Video Downloader';
const appVersion = typeof __APP_VERSION__ === 'undefined' ? '' : __APP_VERSION__;

const routeName = computed(() => String(route.name ?? ''));
const isSettings = computed(() => routeName.value.startsWith('settings'));
const isQueue = computed(() => routeName.value === 'home'
  || routeName.value.startsWith('group')
  || routeName.value.startsWith('preferences'));

const hasAuthConfigured = computed(() => settingsStore.hasAuthConfigured() || strongholdStore.hasAvailableKeys());
const hasSubtitles = computed(() => settingsStore.settings.subtitles.enabled || settingsStore.settings.translation.enabled);
const hasFilters = computed(() => isInputFiltersActive(settingsStore.settings.inputFilters));
const isDownloading = computed(() => stateStore.hasGroupWithState(MediaState.downloading, MediaState.downloadingList));

const mainItems = computed<NavItem[]>(() => [
  {
    key: 'queue',
    to: { name: 'home' },
    icon: QueueListIcon,
    label: t('layout.sidebar.queue'),
    active: isQueue.value,
    badge: groupStore.countGroups(),
    badgeActive: isDownloading.value,
  },
  {
    key: 'location',
    to: { name: 'location' },
    icon: FolderIcon,
    label: t('layout.sidebar.location'),
    active: routeName.value === 'location',
  },
  {
    key: 'authentication',
    to: { name: 'authentication' },
    icon: KeyIcon,
    label: t('layout.sidebar.authentication'),
    active: routeName.value === 'authentication',
    on: hasAuthConfigured.value,
  },
  {
    key: 'subtitles',
    to: { name: 'subtitles' },
    icon: ChatBubbleBottomCenterTextIcon,
    label: t('layout.sidebar.subtitles'),
    active: routeName.value === 'subtitles',
    on: hasSubtitles.value,
  },
  {
    key: 'filters',
    to: { name: 'input-filters' },
    icon: FunnelIcon,
    label: t('layout.sidebar.filters'),
    active: routeName.value === 'input-filters',
    on: hasFilters.value,
  },
]);

function itemClass(active: boolean): string[] {
  return [
    'flex h-10 items-center gap-2.5 rounded-lg px-2.5 text-sm transition-colors duration-150',
    'focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary',
    active
      ? 'bg-base-100 font-semibold text-base-content shadow-xs'
      : 'text-base-content/75 hover:bg-base-content/5 hover:text-base-content',
  ];
}
</script>
