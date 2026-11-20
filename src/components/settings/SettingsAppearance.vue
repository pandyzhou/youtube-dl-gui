<template>
  <base-fieldset
      :legend="t('settings.appearance.legend')"
      :label="t('settings.appearance.legendLabel')"
      rows
  >
    <base-setting-row :label="t('settings.appearance.theme.label')" label-for="theme">
      <select
          id="theme"
          v-model="settings.appearance.theme"
          class="select w-56"
      >
        <option value="system">
          {{
            t('settings.appearance.theme.options.system', {
              theme: systemTheme === 'dark'
                  ? t('settings.appearance.theme.options.dark')
                  : t('settings.appearance.theme.options.light')
            })
          }}
        </option>
        <option value="light">{{ t('settings.appearance.theme.options.light') }}</option>
        <option value="dark">{{ t('settings.appearance.theme.options.dark') }}</option>
      </select>
    </base-setting-row>
    <base-setting-row :label="t('settings.appearance.language.label')" label-for="language">
      <select
          id="language"
          v-model="settings.appearance.language"
          class="select w-56"
      >
        <option value="system">{{ t('settings.appearance.language.options.system', { language: systemLocale }) }}</option>
        <option v-for="option in languageOptions()" :key="option.value" :value="option.value">{{ option.label }}</option>
      </select>
    </base-setting-row>
    <base-setting-row
        :label="t('settings.appearance.expandedOptions.label')"
        :hint="t('settings.appearance.expandedOptions.hint')"
        label-for="expandedOptions"
    >
      <select
          id="expandedOptions"
          v-model="settings.appearance.expandedOptions"
          class="select w-56"
      >
        <option value="none">{{ t('settings.appearance.expandedOptions.options.none') }}</option>
        <option value="encodings">{{ t('settings.appearance.expandedOptions.options.encodings') }}</option>
        <option value="tracks">{{ t('settings.appearance.expandedOptions.options.tracks') }}</option>
      </select>
    </base-setting-row>
  </base-fieldset>
</template>

<script setup lang="ts">
import BaseFieldset from '../base/BaseFieldset.vue';
import BaseSettingRow from '../base/BaseSettingRow.vue';
import { Settings } from '../../tauri/types/config.ts';
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import { SelectOption } from '../../helpers/forms.ts';
import { useTheme } from '../../composables/useTheme.ts';
import { availableLocales, getDefaultLocale } from '../../i18n.ts';
import { languageOptionsLookup, SubtitleLanguageOption } from '../../helpers/subtitles/languages.ts';

const { t } = useI18n();
const settings = defineModel<Settings>({ required: true });

const { systemTheme } = useTheme();

const languageOptions = () => {
  const options: SelectOption[] = [];
  for (const locale of Object.keys(availableLocales)) {
    const language: SubtitleLanguageOption | undefined = languageOptionsLookup.get(locale);
    if (language) {
      if (language.nativeName && language.nativeName !== language.englishName) {
        options.push({
          label: `${language.englishName} - ${language.nativeName}`,
          value: language.code,
        });
      } else {
        options.push({ label: language.englishName, value: language.code });
      }
    }
  }
  options.sort((a, b) => a.label.localeCompare(b.label));
  return options;
};

const systemLocale = computed(() => {
  const defaultLocale = getDefaultLocale();
  const language = languageOptionsLookup.get(defaultLocale);
  if (language) {
    return language.nativeName;
  }
  return defaultLocale;
});
</script>
