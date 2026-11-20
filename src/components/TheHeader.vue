<template>
  <header class="relative z-10 border-b border-line bg-base-200 px-6 pt-5 pb-4">
    <input
        ref="fileInput"
        type="file"
        accept=".csv,.txt,text/csv,text/plain"
        class="hidden"
        @change="handleFileSelection"
    />
    <div class="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <h1 class="text-xl font-semibold">{{ t('layout.header.title') }}</h1>
      <p class="text-sm text-base-content/60">{{ t('layout.header.subtitle') }}</p>
    </div>
    <form @submit.prevent="handleSubmit" class="flex gap-2" autocomplete="off">
      <label class="input h-11 min-w-0 flex-1 gap-2.5 text-[15px]">
        <link-icon class="size-5 shrink-0 text-base-content/50" aria-hidden="true" />
        <span class="sr-only">{{ t('layout.header.inputLabel') }}</span>
        <input
            v-model="url"
            id="queue-url-input"
            name="URL input to queue a video or playlist"
            class="grow"
            :placeholder="inputPlaceholder"
            type="text"
            inputmode="url"
            autocomplete="off"
            autocorrect="off"
            autocapitalize="off"
            spellcheck="false"
            ref="input"
        />
      </label>
      <button
          type="button"
          class="btn btn-primary h-11 gap-1.5 px-5 whitespace-nowrap"
          :disabled="isInputDisabled"
          @click="handleAddClick"
      >
        <plus-icon class="size-5" aria-hidden="true" />
        {{ t('common.add') }}
      </button>
    </form>
    <div class="mt-3 flex flex-wrap gap-2">
      <button
          type="button"
          :class="chipClass(watchClipboardStore.isActive)"
          :aria-pressed="watchClipboardStore.isActive"
          @click="watchClipboardStore.toggle()"
      >
        <clipboard-document-check-icon v-if="watchClipboardStore.isActive" class="size-4" aria-hidden="true" />
        <clipboard-document-list-icon v-else class="size-4" aria-hidden="true" />
        {{ t('layout.header.actions.watchClipboardStart') }}
      </button>
      <button type="button" :class="chipClass(false)" @click="handleImportClick">
        <document-arrow-up-icon class="size-4" aria-hidden="true" />
        {{ t('layout.header.actions.importFile') }}
      </button>
      <button
          type="button"
          :class="chipClass(hasActiveInputFilters)"
          @click="openInputFilters"
      >
        <funnel-icon-solid v-if="hasActiveInputFilters" class="size-4" aria-hidden="true" />
        <funnel-icon v-else class="size-4" aria-hidden="true" />
        {{ t('layout.header.actions.inputFilters') }}
        <span v-if="hasActiveInputFilters" class="text-xs opacity-80">{{ t('layout.header.actions.filtersOn') }}</span>
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">

import {
  ClipboardDocumentCheckIcon,
  ClipboardDocumentListIcon,
  DocumentArrowUpIcon,
  FunnelIcon,
  LinkIcon,
  PlusIcon,
} from '@heroicons/vue/24/outline';
import { FunnelIcon as FunnelIconSolid } from '@heroicons/vue/24/solid';
import { useMediaStore } from '../stores/media/media';
import { ref, computed, onMounted, watch } from 'vue';
import { useClipboard } from '../composables/useClipboard';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useSettingsStore } from '../stores/settings';
import { isValidUrl } from '../helpers/url.ts';
import { useWatchClipboardStore } from '../stores/watchClipboard.ts';
import { useToastStore } from '../stores/toast.ts';
import {
  getUrlImportReadErrorToast,
  getUrlImportToast,
  isSupportedImportFile,
  parseUrlFileText,
  parseUrlInputText,
} from '../helpers/urlImport.ts';
import { isInputFiltersActive } from '../helpers/inputFilters.ts';

const { t } = useI18n();
const router = useRouter();
const mediaStore = useMediaStore();
const toastStore = useToastStore();

const settingsStore = useSettingsStore();
const watchClipboardStore = useWatchClipboardStore();

const doPolling = computed(() => settingsStore.settings.input.autoFillClipboard || watchClipboardStore.isActive);
const hasActiveInputFilters = computed(() => isInputFiltersActive(settingsStore.settings.inputFilters));

const { content: clipboardContent, poll } = useClipboard({
  doPolling,
});

const input = ref<HTMLInputElement | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const fileImportImmediateDownload = ref(false);

const inputPlaceholder = computed(() => {
  if (watchClipboardStore.isActive) {
    return t('layout.header.watchClipboardPlaceholder');
  }
  const defaultPlaceholder = t('layout.header.placeholder');
  if (clipboardHasValidUrl.value) {
    return clipboardContent.value ?? defaultPlaceholder;
  } else {
    return defaultPlaceholder;
  }
});

const clipboardHasValidUrl = computed(() => isValidUrl(clipboardContent));

const isInputDisabled = computed(() => {
  return url.value.length === 0 && !clipboardHasValidUrl.value;
});

const url = ref('');

function chipClass(active: boolean): string[] {
  return [
    'btn btn-sm h-8 gap-1.5 rounded-full px-3 font-normal shadow-none whitespace-nowrap',
    active
      ? 'border-primary/60 bg-primary/10 text-primary'
      : 'btn-ghost border-line text-base-content/75 hover:text-base-content',
  ];
}

async function addClipboardUrlToQueue(urlToRecord: string) {
  if (!watchClipboardStore.isActive || !isValidUrl(urlToRecord) || watchClipboardStore.hasSeen(urlToRecord)) {
    return;
  }

  watchClipboardStore.markSeen(urlToRecord);
  await mediaStore.dispatchMediaInfoFetch(urlToRecord);
}

function addFromInput(immediateDownload: boolean = false) {
  const urlToSubmit = url.value.length > 0 ? url.value : clipboardContent.value;
  if (!urlToSubmit) return;
  void processParsedUrls(parseUrlInputText(urlToSubmit), immediateDownload, true);
  void router.push('/');
  url.value = '';
}

async function processParsedUrls(
  result: { urls: string[]; skipped: number },
  immediateDownload: boolean = false,
  fromInput: boolean = false,
) {
  if (result.urls.length > 0) {
    if (immediateDownload) {
      await mediaStore.addUrlBatchAndDownload(result.urls, false, true);
    } else {
      await mediaStore.addUrlBatch(result.urls);
    }
  }

  if (!fromInput || result.urls.length > 1) {
    const toast = getUrlImportToast(result);
    toastStore.showToast(toast.message, { style: toast.style });
  }
}

function handleSubmit() {
  addFromInput();
}

function handleAddClick(event: MouseEvent) {
  addFromInput(event.shiftKey);
}

function handleImportClick(event: MouseEvent) {
  fileImportImmediateDownload.value = event.shiftKey;
  fileInput.value?.click();
}

function openInputFilters() {
  void router.push({ name: 'input-filters' });
}

async function handleFileSelection(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  target.value = '';

  if (!file || !isSupportedImportFile(file)) {
    const toast = getUrlImportToast({ urls: [], skipped: 0 });
    toastStore.showToast(toast.message, { style: toast.style });
    return;
  }

  try {
    const text = await file.text();
    await processParsedUrls(parseUrlFileText(text), fileImportImmediateDownload.value);
  } catch {
    const toast = getUrlImportReadErrorToast();
    toastStore.showToast(toast.message, { style: toast.style });
  } finally {
    fileImportImmediateDownload.value = false;
  }
}

onMounted(() => {
  input.value?.focus();
});

watch(clipboardContent, (value) => {
  if (!value) return;
  void addClipboardUrlToQueue(value);
});

watch(() => watchClipboardStore.isActive, (isActive) => {
  if (!isActive) return;
  poll();
  const currentClipboard = clipboardContent.value;
  if (!currentClipboard) return;
  void addClipboardUrlToQueue(currentClipboard);
});

</script>
