<template>
  <div class="absolute top-1/2 left-1/2 flex w-full max-w-md -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-3 px-6 text-center">
    <div class="mb-1 flex size-16 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-1 ring-primary/40">
      <cloud-arrow-down-icon class="size-8" aria-hidden="true" />
    </div>
    <p class="text-lg font-semibold">
      {{ t('components.base.dragState.title') }}
    </p>
    <p class="text-sm text-base-content/60">
      {{ t('components.base.dragState.subtitle') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { CloudArrowDownIcon } from '@heroicons/vue/24/outline';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
</script>
