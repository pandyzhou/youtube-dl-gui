<template>
  <div class="flex w-full min-w-0 flex-col gap-1.5">
    <label
        v-if="$slots.default"
        :for="composedId"
        class="min-w-0 truncate text-sm font-medium"
    >
      <slot/>
    </label>
    <progress
        :id="composedId"
        class="progress h-2 w-full [&::-webkit-progress-value]:transition-all [&::-moz-progress-bar]:transition-all motion-reduce:[&::-webkit-progress-value]:transition-none"
        :class="progressClass"
        :max="max"
        :value="value"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, PropType } from 'vue';

type ProgressVariant = 'primary' | 'secondary' | 'success' | 'warning' | 'error';

const PROGRESS_VARIANT_CLASS: Record<ProgressVariant, string> = {
  primary: 'progress-primary',
  secondary: 'progress-secondary',
  success: 'progress-success',
  warning: 'progress-warning',
  error: 'progress-error',
};

const composedId = computed(() => `${id}-progress`);

const { id, max, value, variant } = defineProps({
  id: {
    type: String,
    required: true,
  },
  max: {
    type: Number,
  },
  value: {
    type: Number,
  },
  variant: {
    type: String as PropType<ProgressVariant>,
    default: 'primary',
  },
});

const progressClass = computed(() => PROGRESS_VARIANT_CLASS[variant]);

</script>
