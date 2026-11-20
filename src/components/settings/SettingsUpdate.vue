<template>
  <base-fieldset
      :legend="t('settings.update.legend')"
      :label="t('settings.update.legendLabel')"
      rows
  >
    <base-setting-row
        :label="t('settings.update.updateBinaries.label')"
        :hint="t('settings.update.updateBinaries.hint')"
        label-for="updateBinaries"
    >
      <input
          id="updateBinaries"
          type="checkbox"
          v-model="settings.update.updateBinaries"
          class="toggle toggle-primary"
      />
    </base-setting-row>
    <base-setting-row
        :label="t('settings.update.updateApp.label')"
        :hint="t('settings.update.updateApp.hint')"
        label-for="updateApp"
    >
      <input
          id="updateApp"
          type="checkbox"
          v-model="settings.update.updateApp"
          class="toggle toggle-primary"
      />
    </base-setting-row>
  </base-fieldset>
</template>
<script setup lang="ts">
import BaseFieldset from '../base/BaseFieldset.vue';
import BaseSettingRow from '../base/BaseSettingRow.vue';
import { Settings } from '../../tauri/types/config.ts';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const settings = defineModel<Settings>({ required: true });
</script>
