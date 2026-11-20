<template>
  <header class="sticky top-0 z-20 flex min-h-16 w-full flex-wrap items-center gap-x-4 gap-y-2 border-b border-line bg-base-200/95 px-6 py-3 backdrop-blur-sm">
    <router-link v-if="backButton" :to="{ name: 'home' }" class="btn btn-ghost btn-sm -ml-2 gap-1 px-2">
      <chevron-left-icon class="size-4" aria-hidden="true"/>
      {{ t('common.back') }}
    </router-link>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-2">
      <slot name="title"/>
    </div>
    <div class="flex shrink-0 items-center gap-2">
      <slot/>
    </div>
  </header>
</template>
<script setup lang="ts">
import { ChevronLeftIcon } from '@heroicons/vue/24/outline';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const { backButton } = defineProps({
  backButton: {
    type: Boolean,
    required: false,
    default: false,
  },
});
</script>
