<template>
  <base-fieldset
    :legend="t('settings.output.legend')"
    :label="t('settings.output.legendLabel')"
  >
    <output-settings-editor v-model="outputState">
      <template #after-common>
        <div class="mt-3 flex flex-wrap gap-2">
          <router-link class="btn btn-soft btn-sm gap-1" :to="{ name: 'location' }">
            {{ t('settings.output.location.link') }}
            <chevron-right-icon class="size-4" aria-hidden="true" />
          </router-link>
          <router-link class="btn btn-soft btn-sm gap-1" :to="{ name: 'subtitles' }">
            {{ t('settings.output.subtitles.link') }}
            <chevron-right-icon class="size-4" aria-hidden="true" />
          </router-link>
        </div>
      </template>
    </output-settings-editor>
  </base-fieldset>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import BaseFieldset from '../base/BaseFieldset.vue';
import OutputSettingsEditor from '../output/OutputSettingsEditor.vue';
import { Settings } from '../../tauri/types/config';
import { ChevronRightIcon } from '@heroicons/vue/24/outline';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const settings = defineModel<Settings>({ required: true });

const outputState = computed({
  get: () => settings.value.output,
  set: (value) => {
    settings.value.output = value;
  },
});
</script>
