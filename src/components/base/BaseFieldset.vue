<template>
  <fieldset class="min-w-0" :class="flat ? '' : 'rounded-box border border-line bg-base-100 px-5 pt-4 pb-5'">
    <legend class="float-left flex w-full flex-wrap items-center gap-2 font-semibold" :class="flat ? 'text-sm' : 'text-[15px]'">
      {{ legend }}
      <span v-if="badge" class="badge badge-soft badge-info badge-sm">{{ badge }}</span>
    </legend>
    <p v-if="label" class="clear-both pt-1 text-sm text-base-content/60">{{ label }}</p>
    <div
        class="settings-body clear-both mt-3 text-sm"
        :class="rows ? 'flex flex-col divide-y divide-line' : 'grid grid-cols-1 auto-rows-max gap-1.5'"
    >
      <slot/>
    </div>
  </fieldset>
</template>

<script setup lang="ts">

const { label, legend, rows, flat } = defineProps({
  label: {
    type: String,
    required: true,
  },
  legend: {
    type: String,
    required: true,
  },
  badge: {
    type: String,
    required: false,
  },
  /** Lay the content out as divided rows (BaseSettingRow) instead of a stack. */
  rows: {
    type: Boolean,
    default: false,
  },
  /** Drop the card frame, for sections nested inside another card. */
  flat: {
    type: Boolean,
    default: false,
  },
});

</script>
