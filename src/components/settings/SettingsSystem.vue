<template>
  <base-fieldset
      :legend="t('settings.system.legend')"
      :label="t('settings.system.legendLabel')"
      rows
  >
    <base-setting-row
        :label="isMac ? t('settings.system.trayEnabled.label.mac') : t('settings.system.trayEnabled.label.generic')"
        :hint="isMac ? t('settings.system.trayEnabled.hint.mac') : t('settings.system.trayEnabled.hint.generic')"
        label-for="trayEnabled"
    >
      <input
          id="trayEnabled"
          type="checkbox"
          v-model="settings.system.trayEnabled"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row
        v-if="!isMac"
        :label="t('settings.system.minimiseToTray.label')"
        :hint="t('settings.system.minimiseToTray.hint')"
        :disabled="settings.system.trayEnabled === false"
        label-for="minimiseToTray"
    >
      <input
          :disabled="settings.system.trayEnabled === false"
          id="minimiseToTray"
          type="checkbox"
          v-model="minimiseToTray"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row
        :label="t('settings.system.autoStart.label')"
        :hint="t('settings.system.autoStart.hint')"
        label-for="autoStart"
    >
      <input
          id="autoStart"
          type="checkbox"
          v-model="settings.system.autoStartEnabled"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row
        :label="autoStartMinimisedLabel"
        :hint="autoStartMinimisedHint"
        :disabled="!settings.system.autoStartEnabled || (!isMac && !settings.system.trayEnabled)"
        label-for="autoStartMinimised"
    >
      <input
          :disabled="!settings.system.autoStartEnabled || (!isMac && !settings.system.trayEnabled)"
          id="autoStartMinimised"
          type="checkbox"
          v-model="settings.system.autoStartMinimised"
          class="toggle toggle-primary"
      />
    </base-setting-row>
  </base-fieldset>
</template>

<script setup lang="ts">
import BaseFieldset from '../base/BaseFieldset.vue';
import BaseSettingRow from '../base/BaseSettingRow.vue';
import { CloseBehavior, Settings } from '../../tauri/types/config.ts';
import { useI18n } from 'vue-i18n';
import { computed, watch } from 'vue';
import { usePlatform } from '../../composables/usePlatform.ts';

const { t } = useI18n();
const { isWindows, isMac } = usePlatform();

const settings = defineModel<Settings>({ required: true });

const autoStartMinimisedLabel = computed(() => isMac.value
  ? t('settings.system.autoStartMinimised.label.mac')
  : t('settings.system.autoStartMinimised.label.windows'));

const autoStartMinimisedHint = computed(() => {
  if (isMac.value) return t('settings.system.autoStartMinimised.hint.mac');
  if (isWindows.value) return t('settings.system.autoStartMinimised.hint.windows');
  return t('settings.system.autoStartMinimised.hint.generic');
});
const minimiseToTray = computed<boolean>({
  get: () => settings.value.system.closeBehavior === CloseBehavior.Hide && settings.value.system.trayEnabled === true,
  set: (value) => {
    settings.value.system.closeBehavior = value ? CloseBehavior.Hide : CloseBehavior.Exit;
  },
});

watch(() => settings.value.system.trayEnabled, (newVal, oldVal) => {
  if (oldVal && !newVal) {
    minimiseToTray.value = false;
  }
  if (!isMac.value) {
    settings.value.system.autoStartMinimised = false;
  }
});

watch(() => settings.value.system.autoStartEnabled, (newVal, oldVal) => {
  if (oldVal && !newVal) {
    settings.value.system.autoStartMinimised = false;
  }
});
</script>
