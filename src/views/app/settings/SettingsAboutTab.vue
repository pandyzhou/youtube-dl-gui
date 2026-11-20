<template>
  <base-fieldset :legend="t('about.title')" label="">
    <div class="flex flex-col gap-4 text-sm">
      <div class="flex items-center gap-3">
        <img :src="iconUrl" alt="" class="size-12 shrink-0 rounded-xl" />
        <div class="min-w-0">
          <p class="text-base font-semibold">{{ appName }}</p>
          <p class="text-base-content/60">{{ t('about.version', { version: appVersion }) }}</p>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <a href="https://github.com/jely2002/youtube-dl-gui" class="btn btn-soft btn-sm" target="_blank" rel="noopener">
          {{ t('about.links.github') }}
        </a>
        <a href="https://github.com/jely2002/youtube-dl-gui/wiki" class="btn btn-soft btn-sm" target="_blank" rel="noopener">
          {{ t('about.links.wiki') }}
        </a>
        <a href="https://github.com/pandyzhou/youtube-dl-gui/issues/new?template=bug_report.md" class="btn btn-soft btn-sm" target="_blank" rel="noopener">
          {{ t('about.links.reportABug') }}
        </a>
      </div>
      <p class="text-base-content/70">{{ t('about.credit') }}</p>
      <div class="flex flex-wrap items-center gap-2 text-base-content/70">
        <p>{{ t('about.poweredBy') }}</p>
        <button type="button" @click="openLicenses()" class="btn btn-ghost btn-xs">
          {{ t('about.viewLicenses') }}
        </button>
      </div>
    </div>
  </base-fieldset>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import BaseFieldset from '../../../components/base/BaseFieldset.vue';
import { useOpener } from '../../../composables/useOpener.ts';

defineOptions({
  inheritAttrs: false,
});

const { t } = useI18n();
const { openInternalPath } = useOpener();
const appName = 'Open Video Downloader';
// Served from public/. Bound (not a static src) so the SFC compiler does not turn it into an import.
const iconUrl = '/icon.png';

const appVersion = typeof __APP_VERSION__ === 'undefined' ? '' : __APP_VERSION__;

const openLicenses = async () => {
  await openInternalPath('licenses/3rdpartylicenses.txt');
};
</script>
