<template>
  <div class="flex flex-col gap-5">
    <SettingsOutput v-model="settings" />
    <SettingsPerformance v-model="settings" />
    <SettingsSponsorBlock v-model="settings" />
  </div>
</template>

<script setup lang="ts">
import { Settings } from '../../../tauri/types/config.ts';
import SettingsOutput from '../../../components/settings/SettingsOutput.vue';
import SettingsPerformance from '../../../components/settings/SettingsPerformance.vue';
import SettingsSponsorBlock from '../../../components/settings/SettingsSponsorBlock.vue';

const settings = defineModel<Settings>({ required: true });
</script>
