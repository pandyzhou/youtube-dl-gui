<template>
  <base-fieldset
      :legend="t('settings.input.legend')"
      :label="t('settings.input.legendLabel')"
      rows
  >
    <base-setting-row
        :label="t('settings.input.autoFillClipboard.label')"
        :hint="t('settings.input.autoFillClipboard.hint')"
        label-for="autoFillClipboard"
    >
      <input
          id="autoFillClipboard"
          type="checkbox"
          v-model="settings.input.autoFillClipboard"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row
        :label="t('settings.input.preferVideoInMixedLinks.label')"
        :hint="t('settings.input.preferVideoInMixedLinks.hint')"
        label-for="preferVideoInMixedLinks"
    >
      <input
          id="preferVideoInMixedLinks"
          type="checkbox"
          v-model="settings.input.preferVideoInMixedLinks"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row :label="t('settings.input.globalShortcuts.label')" label-for="globalShortcuts">
      <template #hint>
        <ul class="mt-1.5 flex flex-col gap-1 text-sm text-base-content/60" :class="{ 'opacity-60': settings.input.globalShortcuts === false }">
          <li v-for="shortcut in shortcuts" :key="shortcut.hint" class="flex flex-wrap items-center gap-1.5">
            {{ shortcut.hint }}
            <span class="flex items-center gap-1">
              <kbd v-for="key in shortcut.keys" :key="key" class="kbd kbd-sm">{{ key }}</kbd>
            </span>
          </li>
        </ul>
      </template>
      <input
          id="globalShortcuts"
          type="checkbox"
          v-model="settings.input.globalShortcuts"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row :label="t('settings.input.authentication.label')">
      <router-link class="btn btn-soft btn-sm gap-1" :to="{ name: 'authentication' }">
        {{ t('settings.input.authentication.link') }}
        <chevron-right-icon class="size-4" aria-hidden="true"/>
      </router-link>
    </base-setting-row>
  </base-fieldset>
</template>
<script setup lang="ts">
import { ChevronRightIcon } from '@heroicons/vue/24/outline';
import { computed } from 'vue';
import BaseFieldset from '../base/BaseFieldset.vue';
import BaseSettingRow from '../base/BaseSettingRow.vue';
import { Settings } from '../../tauri/types/config.ts';
import { useI18n } from 'vue-i18n';
import { usePlatform } from '../../composables/usePlatform.ts';

const { t } = useI18n();
const { isMac } = usePlatform();
const settings = defineModel<Settings>({ required: true });

const shortcuts = computed(() => {
  const modifier = isMac.value ? 'ctrl' : 'alt';
  return [
    { hint: t('settings.input.globalShortcuts.add_hint'), keys: [modifier, 'shift', 'V'] },
    { hint: t('settings.input.globalShortcuts.download_hint'), keys: [modifier, 'shift', 'Enter'] },
    { hint: t('settings.input.globalShortcuts.instant_hint'), keys: [modifier, 'shift', 'D'] },
  ];
});
</script>
