<template>
  <base-fieldset
      :legend="t('settings.performance.legend')"
      :label="t('settings.performance.legendLabel')"
      rows
  >
    <base-setting-row :label="t('settings.performance.maxConcurrency.label')" label-for="maxConcurrency">
      <input
          v-model.number="settings.performance.maxConcurrency"
          type="range"
          :min="1"
          :max="32"
          id="maxConcurrency"
          class="range range-primary range-sm w-44"
      />
      <span class="w-6 text-right tabular-nums">{{ settings.performance.maxConcurrency }}</span>
    </base-setting-row>
    <base-setting-row
        :label="t('settings.performance.concurrentFragments.label')"
        :hint="t('settings.performance.concurrentFragments.hint')"
        label-for="concurrentFragments"
    >
      <input
          v-model.number="settings.performance.concurrentFragments"
          type="range"
          :min="1"
          :max="16"
          id="concurrentFragments"
          class="range range-primary range-sm w-44"
      />
      <span class="w-6 text-right tabular-nums">{{ settings.performance.concurrentFragments }}</span>
    </base-setting-row>
    <base-setting-row :label="t('settings.performance.splitPlaylistThreshold.label')" label-for="splitPlaylistThreshold">
      <select
          id="splitPlaylistThreshold"
          v-model="settings.performance.splitPlaylistThreshold"
          class="select w-56"
      >
        <option :value="1">{{ t('settings.performance.splitPlaylistThreshold.options.never') }}</option>
        <option v-for="interval in splitIntervals" :key="interval" :value="interval">
          {{ t('settings.performance.splitPlaylistThreshold.options.lessThan', { number: interval }) }}
        </option>
      </select>
    </base-setting-row>
    <base-setting-row :label="t('settings.performance.autoLoadSize.label')" label-for="autoLoadSize">
      <input
          id="autoLoadSize"
          type="checkbox"
          v-model="settings.performance.autoLoadSize"
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

const splitIntervals = [50, 100, 150, 200];
</script>
