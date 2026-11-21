<template>
  <div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 py-3.5 first:pt-1 last:pb-0">
    <div class="min-w-0 flex-1 basis-60">
      <label v-if="labelFor" :for="labelFor" class="text-sm font-medium" :class="{ 'opacity-60': disabled }">{{ text }}</label>
      <p v-else class="text-sm font-medium" :class="{ 'opacity-60': disabled }">{{ text }}</p>
      <p v-if="hint" class="mt-0.5 text-sm leading-5 text-base-content/60">{{ hint }}</p>
      <slot name="hint" />
    </div>
    <div class="flex max-w-full shrink-0 items-center gap-3">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  label: string;
  hint?: string;
  /** Id of the control this row describes, so clicking the label focuses it. */
  labelFor?: string;
  disabled?: boolean;
}>();

// Many labels were written for a stacked layout and end in a colon ("Theme:",
// "主题："), which reads oddly when the control sits on the right of the row.
const text = computed(() => props.label.replace(/\s*[:：]\s*$/, ''));
</script>
