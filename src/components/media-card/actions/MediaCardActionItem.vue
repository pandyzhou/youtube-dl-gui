<template>
  <button
      v-if="!to"
      type="button"
      :disabled="disabled"
      :title="label"
      class="btn btn-ghost btn-square btn-sm text-base-content/60 hover:text-base-content disabled:bg-transparent"
  >
    <span class="sr-only">{{ label }}</span>
    <component :is="icon" class="size-[18px]" aria-hidden="true"/>
  </button>
  <router-link
      v-else
      :disabled="disabled ? true : undefined"
      :title="label"
      :to="to"
      class="btn btn-ghost btn-square btn-sm text-base-content/60 hover:text-base-content"
  >
    <span class="sr-only">{{ label }}</span>
    <component :is="icon" class="size-[18px]" aria-hidden="true"/>
  </router-link>
</template>
<script setup lang="ts">
import { type FunctionalComponent, type HTMLAttributes, PropType, type VNodeProps } from 'vue';
import { RouteLocationRaw } from 'vue-router';

const { icon, label, to } = defineProps({
  icon: {
    type: Function as PropType<FunctionalComponent<HTMLAttributes & VNodeProps>>,
    required: true,
  },
  label: {
    type: String,
    required: true,
  },
  to: {
    type: [String, Object] as PropType<RouteLocationRaw>,
    required: false,
  },
  disabled: {
    type: Boolean,
    required: false,
    default: false,
  },
});
</script>
