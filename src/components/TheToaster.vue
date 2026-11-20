<template>
  <div class="toast toast-end toast-bottom fixed right-2 bottom-28 z-50 max-w-sm">
    <TransitionGroup name="toast-fade" tag="div">
      <div
          v-for="toast in toasts"
          :key="toast.id"
          class="alert mb-2 cursor-pointer border-line text-sm whitespace-normal shadow-lg"
          :class="styleClass(toast.style)"
          @click="removeToast(toast.id)"
      >
        <span>{{ toast.message }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">

import { ToastStyle, useToastStore } from '../stores/toast';

const { toasts, removeToast } = useToastStore();

const styleClass = (style: ToastStyle) => {
  return {
    'alert-success': style === 'success',
    'alert-info': style === 'info',
    'alert-error': style === 'error',
    'alert-warning': style === 'warning',
    'bg-base-100': style === 'subtle',
  };
};
</script>

<style scoped>
.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: all 0.3s ease;
}
.toast-fade-enter-from {
  opacity: 0;
  transform: translateY(-10px);
}
.toast-fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
