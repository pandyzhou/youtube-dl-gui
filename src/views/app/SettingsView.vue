<template>
  <form @submit.prevent="saveSettings">
    <base-sub-nav>
      <template #default>
        <base-button-dropdown
            placement="bottom"
            align="end"
            btnClass="btn-primary"
            :mainDisabled="!hasChanges || isResetting"
            :mainLoading="isSaving"
            :caretAriaLabel="t('settings.reset.more')"
            mainType="submit"
        >
          <template #main>
            {{ t('common.save') }}
          </template>

          <li>
            <button type="button" class="gap-2 text-nowrap" @click="resetSettings">
              {{ t('settings.reset.label') }}
            </button>
          </li>
        </base-button-dropdown>
      </template>
      <template v-slot:title>
        <h1 class="text-xl font-semibold">{{ t('settings.title') }}</h1>
        <div role="tablist" class="seg-tabs">
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'settings.downloads' }" class="seg-tab">
            {{ t('settings.tabs.downloads') }}
          </router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'settings.app' }" class="seg-tab">
            {{ t('settings.tabs.app') }}
          </router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'settings.network' }" class="seg-tab">
            {{ t('settings.tabs.network') }}
          </router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'settings.system' }" class="seg-tab">
            {{ t('settings.tabs.system') }}
          </router-link>
          <router-link exactActiveClass="tab-active" role="tab" :to="{ name: 'settings.about' }" class="seg-tab">
            {{ t('settings.tabs.about') }}
          </router-link>
        </div>
      </template>
    </base-sub-nav>
    <section class="mx-auto flex w-full max-w-3xl flex-col gap-5 px-6 py-6">
      <router-view v-slot="{ Component }">
        <component :is="Component" v-model="draft" />
      </router-view>
    </section>
  </form>
</template>

<script setup lang="ts">
import { computed, ref, toRaw } from 'vue';
import { useI18n } from 'vue-i18n';
import BaseSubNav from '../../components/base/BaseSubNav.vue';
import { useSettingsStore } from '../../stores/settings';
import { useToastStore } from '../../stores/toast';
import { useTheme } from '../../composables/useTheme.ts';
import { Settings } from '../../tauri/types/config.ts';
import BaseButtonDropdown from '../../components/base/BaseButtonDropdown.vue';

const settingsStore = useSettingsStore();
const toastStore = useToastStore();
const { setTheme } = useTheme();
const { t } = useI18n();

const draft = ref<Settings>(structuredClone<Settings>(toRaw(settingsStore.settings)));

const isSaving = ref(false);
const isResetting = ref(false);
const hasChanges = computed(() => JSON.stringify(draft.value) !== JSON.stringify(settingsStore.settings));

const saveSettings = async () => {
  isSaving.value = true;
  await settingsStore.patch(draft.value);
  if (draft.value.appearance.theme) {
    setTheme(draft.value.appearance.theme);
  }
  isSaving.value = false;
  toastStore.showToast(t('settings.toasts.saved'), { style: 'success' });
};

const resetSettings = async () => {
  isResetting.value = true;
  try {
    const newSettings = await settingsStore.reset();
    draft.value = structuredClone<Settings>(toRaw(newSettings));
    if (newSettings.appearance.theme) {
      setTheme(newSettings.appearance.theme);
    }
    toastStore.showToast(t('settings.toasts.reset'), { style: 'success' });
  } finally {
    isResetting.value = false;
  }
};
</script>
