<template>
  <form @submit.prevent="save">
    <base-sub-nav>
      <template #default>
        <base-button
          type="submit"
          class="btn-primary"
          :disabled="!hasChanges"
          :loading="isSaving"
        >
          {{ t('common.save') }}
        </base-button>
      </template>
      <template #title>
        <div class="min-w-0">
          <h1 class="text-xl font-semibold">{{ t('layout.sidebar.subtitles') }}</h1>
          <p class="text-sm text-base-content/60">{{ t('subtitles.pageSubtitle') }}</p>
        </div>
      </template>
    </base-sub-nav>

    <section class="mx-auto flex w-full max-w-3xl flex-col gap-5 px-6 py-6">
      <base-fieldset
        :legend="t('subtitles.options.languages.legend')"
        :label="t('subtitles.options.languages.legendLabel')"
        rows
      >
        <base-setting-row :label="t('subtitles.options.languages.downloadAll')" label-for="all-languages">
          <input
            id="all-languages"
            type="checkbox"
            class="toggle toggle-primary"
            :checked="isAllSelected"
            @change="toggleAllLanguages"
          />
        </base-setting-row>

        <div class="flex flex-col gap-3 py-3.5 last:pb-0" :class="{ 'opacity-60': isAllSelected }">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-medium">{{ t('subtitles.options.languages.selected') }}</span>
            <template v-if="isAllSelected">
              <span class="badge badge-soft badge-info">
                {{ t('subtitles.options.languages.allSelected') }}
              </span>
            </template>
            <template v-else>
              <span
                v-if="selectedLanguageBadges.length === 0"
                class="badge badge-soft"
              >
                {{ t('subtitles.options.languages.none') }}
              </span>
              <span
                v-for="label in selectedLanguageBadges"
                :key="label"
                class="badge badge-soft badge-primary"
              >
                {{ label }}
              </span>
            </template>
          </div>

          <label class="input w-full">
            <magnifying-glass-icon class="size-4 shrink-0 text-base-content/50" aria-hidden="true" />
            <span class="sr-only">{{ t('subtitles.options.languages.search') }}</span>
            <input
              id="languageSearch"
              v-model="languageQuery"
              type="search"
              class="grow"
              :placeholder="t('subtitles.options.languages.searchPlaceholder')"
            />
          </label>

          <ul class="max-h-64 overflow-y-auto rounded-lg border border-line bg-field">
            <li
              v-for="option in filteredLanguageOptions"
              :key="option.code"
              class="border-b border-line last:border-b-0"
            >
              <label class="flex cursor-pointer items-center gap-3 px-3 py-2 hover:bg-base-content/5">
                <input
                  type="checkbox"
                  class="checkbox checkbox-sm checkbox-primary"
                  :checked="isLanguageSelected(option.code)"
                  :disabled="isAllSelected"
                  @change="toggleLanguage(option.code)"
                />
                <span class="flex min-w-0 flex-col">
                  <span class="text-sm">{{ option.englishName }}</span>
                  <span
                    v-if="option.nativeName && option.nativeName !== option.englishName"
                    class="text-xs text-base-content/60"
                  >
                    {{ option.nativeName }}
                  </span>
                </span>
              </label>
            </li>
            <li v-if="!filteredLanguageOptions.length" class="px-4 py-6 text-center text-sm text-base-content/60">
              {{ t('subtitles.options.languages.noResult', { query: languageQuery }) }}
            </li>
          </ul>
        </div>
      </base-fieldset>

      <base-fieldset
        :legend="t('subtitles.options.format.label')"
        :label="t('subtitles.options.format.legendLabel')"
        rows
      >
        <base-setting-row :label="t('subtitles.options.format.label')" label-for="format-select">
          <select
            id="format-select"
            v-model="primaryFormat"
            class="select w-56"
          >
            <option
              v-for="format in subtitleFormatOptions"
              :key="format.value"
              :value="format.value"
            >
              {{ format.label }}
            </option>
          </select>
        </base-setting-row>
        <base-setting-row
          :label="t('subtitles.options.embed.label')"
          :hint="t('subtitles.options.embed.hint')"
          label-for="embed-subtitles"
        >
          <input
            id="embed-subtitles"
            v-model="subtitleSettings.embedSubtitles"
            type="checkbox"
            class="toggle toggle-primary"
          />
        </base-setting-row>
      </base-fieldset>

      <base-fieldset
        :legend="t('subtitles.translation.legend')"
        :label="t('subtitles.translation.legendLabel')"
        rows
      >
        <base-setting-row
          :label="t('subtitles.translation.enable.label')"
          :hint="t('subtitles.translation.enable.hint')"
          label-for="translation-enabled"
        >
          <input
            id="translation-enabled"
            v-model="translationSettings.enabled"
            type="checkbox"
            class="toggle toggle-primary"
          />
        </base-setting-row>

        <div class="grid grid-cols-1 gap-4 py-4 last:pb-0 sm:grid-cols-2">
          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <label class="text-sm font-medium" for="translation-base-url">
              {{ t('subtitles.translation.baseUrl.label') }}
            </label>
            <input
              id="translation-base-url"
              v-model.trim="translationSettings.baseUrl"
              type="url"
              class="input w-full"
              :placeholder="t('subtitles.translation.baseUrl.placeholder')"
              autocomplete="off"
              spellcheck="false"
            />
            <p class="text-xs leading-5 text-base-content/60">
              {{ t('subtitles.translation.baseUrl.hint') }}
            </p>
          </div>

          <div class="flex flex-col gap-1.5">
            <label class="text-sm font-medium" for="translation-model">
              {{ t('subtitles.translation.model.label') }}
            </label>
            <input
              id="translation-model"
              v-model.trim="translationSettings.model"
              type="text"
              class="input w-full"
              :placeholder="t('subtitles.translation.model.placeholder')"
              autocomplete="off"
              spellcheck="false"
            />
          </div>

          <div class="flex flex-col gap-1.5">
            <label class="text-sm font-medium" for="translation-target">
              {{ t('subtitles.translation.target.label') }}
            </label>
            <input
              id="translation-target"
              v-model.trim="translationSettings.targetLanguage"
              type="text"
              class="input w-full"
              autocomplete="off"
            />
          </div>

          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <credentials-init v-if="!strongholdStore.status.unlocked" />
            <template v-else>
              <base-secret-input
                id="translation-api-key"
                v-model="apiKey"
                :password="true"
                :label="t('subtitles.translation.apiKey.label')"
              />
              <p class="flex items-center gap-1.5 text-xs leading-5 text-base-content/60">
                <lock-closed-icon class="size-3.5 shrink-0" aria-hidden="true" />
                {{ t('subtitles.translation.apiKey.hint') }}
              </p>
            </template>
          </div>
        </div>
      </base-fieldset>
    </section>
  </form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import BaseButton from '../../components/base/BaseButton.vue';
import BaseFieldset from '../../components/base/BaseFieldset.vue';
import BaseSubNav from '../../components/base/BaseSubNav.vue';
import BaseSettingRow from '../../components/base/BaseSettingRow.vue';
import { LockClosedIcon, MagnifyingGlassIcon } from '@heroicons/vue/24/outline';
import {
  languageOptions,
  getSubtitleLanguageLabel,
  type SubtitleLanguageOption,
} from '../../helpers/subtitles/languages.ts';
import {
  areSubtitleSettingsEqual,
  buildSubtitleFormatOptions,
  createSubtitleSettings,
  getPrimarySubtitleFormat,
} from '../../helpers/subtitles/settings.ts';
import { sanitizeSubtitleFormats, sanitizeSubtitleLanguages } from '../../helpers/subtitles/sanitize.ts';
import BaseSecretInput from '../../components/base/BaseSecretInput.vue';
import CredentialsInit from '../../components/authentication/CredentialsInit.vue';
import { useSettingsStore } from '../../stores/settings.ts';
import { TRANSLATION_API_KEY, useStrongholdStore } from '../../stores/stronghold.ts';
import { useToastStore } from '../../stores/toast.ts';
import { useI18n } from 'vue-i18n';
import type { SubtitleSettings, TranslationSettings } from '../../tauri/types/config.ts';

const { t } = useI18n();
const settingsStore = useSettingsStore();
const strongholdStore = useStrongholdStore();
const toastStore = useToastStore();

const subtitleFormatOptions = buildSubtitleFormatOptions(t);

const subtitleSettings = ref<SubtitleSettings>(
  createSubtitleSettings(settingsStore.settings.subtitles),
);
const translationSettings = ref<TranslationSettings>({ ...settingsStore.settings.translation });
const apiKey = ref<string | null>(null);
const savedApiKey = ref<string | null>(null);
const languageQuery = ref('');
const isSaving = ref(false);

const isApiKeyChanged = computed(() => (apiKey.value ?? '') !== (savedApiKey.value ?? ''));

const hasTranslationChanges = computed(() => {
  const current = JSON.stringify(translationSettings.value);
  const saved = JSON.stringify(settingsStore.settings.translation);
  return current !== saved || isApiKeyChanged.value;
});

const hasChanges = computed(() => {
  const subtitlesChanged = !areSubtitleSettingsEqual(subtitleSettings.value, settingsStore.settings.subtitles);
  return subtitlesChanged || hasTranslationChanges.value;
});

async function loadApiKey() {
  if (!strongholdStore.status.unlocked) return;
  try {
    const value = await strongholdStore.getSecret(TRANSLATION_API_KEY);
    apiKey.value = value;
    savedApiKey.value = value;
  } catch (error) {
    console.error(error);
  }
}

watch(
  () => strongholdStore.status.unlocked,
  (unlocked) => {
    if (unlocked) void loadApiKey();
  },
  { immediate: true },
);

const languageSelection = computed({
  get: () => subtitleSettings.value.languages,
  set: (value: string[]) => {
    subtitleSettings.value.languages = sanitizeSubtitleLanguages(value);
  },
});

const orderedFormats = computed(() =>
  sanitizeSubtitleFormats(subtitleSettings.value.formatPreference),
);

const primaryFormat = computed({
  get: () => orderedFormats.value[0] ?? getPrimarySubtitleFormat(subtitleSettings.value),
  set: (value: string) => {
    subtitleSettings.value.formatPreference = sanitizeSubtitleFormats([value]);
  },
});

const isAllSelected = computed(() => languageSelection.value.includes('all'));

function toggleAllLanguages() {
  languageSelection.value = isAllSelected.value ? [] : ['all'];
}

function toggleLanguage(code: string) {
  const next = new Set(languageSelection.value);
  if (next.has(code)) {
    next.delete(code);
  } else {
    next.add(code);
  }
  next.delete('all');
  languageSelection.value = Array.from(next);
}

function isLanguageSelected(code: string): boolean {
  return languageSelection.value.includes(code);
}

function matchesQuery(option: SubtitleLanguageOption): boolean {
  const query = languageQuery.value.trim().toLowerCase();
  if (!query) {
    return true;
  }

  return (
    option.englishName.toLowerCase().includes(query)
    || option.nativeName.toLowerCase().includes(query)
    || option.code.toLowerCase().includes(query)
  );
}

const filteredLanguageOptions = computed(() =>
  languageOptions.filter(option => matchesQuery(option) || isLanguageSelected(option.code)),
);

const selectedLanguageBadges = computed(() => {
  if (isAllSelected.value) {
    return [];
  }

  return languageSelection.value
    .map(code => getSubtitleLanguageLabel(code))
    .filter((label, index, array) => array.indexOf(label) === index);
});

watch(
  () => settingsStore.settings.subtitles,
  (value) => {
    subtitleSettings.value = createSubtitleSettings(value);
  },
  { deep: true },
);

watch(
  () => settingsStore.settings.translation,
  (value) => {
    translationSettings.value = { ...value };
  },
  { deep: true },
);

async function save() {
  const normalized = createSubtitleSettings(subtitleSettings.value);
  const translation = { ...translationSettings.value };

  isSaving.value = true;
  try {
    await settingsStore.patch({ subtitles: normalized, translation });
    if (strongholdStore.status.unlocked && isApiKeyChanged.value) {
      await strongholdStore.setSecret(TRANSLATION_API_KEY, apiKey.value);
      savedApiKey.value = apiKey.value;
    }
    subtitleSettings.value = normalized;
    toastStore.showToast(t('subtitles.toasts.saved'), { style: 'success' });
  } catch (error) {
    console.error(error);
    toastStore.showToast(t('subtitles.toasts.error', { error: String(error) }), {
      style: 'error',
    });
  } finally {
    isSaving.value = false;
  }
}
</script>
