<template>
  <div class="absolute top-1/2 left-1/2 flex w-full max-w-md -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-3 px-6 text-center">
    <div class="mb-1 flex size-16 items-center justify-center rounded-2xl bg-base-100 text-base-content/40 ring-1 ring-line">
      <film-icon class="size-8" aria-hidden="true" />
    </div>
    <p class="text-lg font-semibold">
      {{ t('components.base.emptyState.title') }}
    </p>
    <p class="text-sm text-base-content/60">
      {{ t('components.base.emptyState.subtitle') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { FilmIcon } from '@heroicons/vue/24/outline';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
</script>
