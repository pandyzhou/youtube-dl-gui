<template>
  <div class="w-full max-w-2xl">
    <label :for="id" v-if="label" class="mb-1.5 block text-sm font-medium">
      {{ label }}
    </label>

    <div class="join w-full">
      <input
          :id="id"
          :type="reveal ? 'text' : (password ? 'password' : 'text')"
          class="input join-item w-full"
          :value="displayValue"
          :disabled="disabled"
          autocomplete="off"
          @input="onInput"
      />
      <button
          v-if="password"
          type="button"
          class="btn btn-soft join-item"
          @click="reveal = !reveal"
      >
        <span class="sr-only">
          {{ reveal ? t('components.base.secretInput.hide') : t('components.base.secretInput.show') }} {{ label }}
        </span>
        <eye-icon v-if="reveal" class="size-5" aria-hidden="true"/>
        <eye-slash-icon v-else class="size-5" aria-hidden="true"/>
      </button>
      <button
          v-if="clearable"
          type="button"
          class="btn btn-soft join-item"
          :disabled="modelValue === null"
          @click="$emit('update:modelValue', null)"
      >
        <span class="sr-only">{{ t('components.base.secretInput.clear', { label }) }}</span>
        <x-circle-icon class="size-5" aria-hidden="true"/>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { XCircleIcon, EyeIcon, EyeSlashIcon } from '@heroicons/vue/24/outline';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = withDefaults(defineProps<{
  modelValue: string | null | undefined;
  id: string;
  label?: string;
  password?: boolean;
  disabled?: boolean;
  clearable?: boolean;
}>(), { password: false, disabled: false, clearable: true });

const emit = defineEmits<{ (event: 'update:modelValue', value: string | null): void }>();

const reveal = ref(false);

const displayValue = computed(() => props.modelValue ?? '');
function onInput(e: Event) {
  const v = (e.target as HTMLInputElement).value;
  emit('update:modelValue', v.trim() === '' ? null : v);
}
</script>
