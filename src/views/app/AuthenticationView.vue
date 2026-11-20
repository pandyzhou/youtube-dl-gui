<template>
  <form @submit.prevent="save">
    <base-sub-nav>
      <template v-slot:default>
        <base-button :disabled="!hasChanges" :loading="isSaving" type="submit" class="btn-primary">
          {{ t('common.save') }}
        </base-button>
      </template>
      <template v-slot:title>
        <div class="min-w-0">
          <h1 class="text-xl font-semibold">{{ t('layout.sidebar.authentication') }}</h1>
          <p class="text-sm text-base-content/60">{{ t('auth.pageSubtitle') }}</p>
        </div>
      </template>
    </base-sub-nav>
    <section class="mx-auto flex w-full max-w-3xl flex-col gap-5 px-6 py-6">
      <cookies-config v-model="cookieFields"/>
      <credentials-config v-model="strongholdFields"/>
    </section>
  </form>
</template>

<script setup lang="ts">
import BaseSubNav from '../../components/base/BaseSubNav.vue';
import BaseButton from '../../components/base/BaseButton.vue';
import { computed, onMounted, ref } from 'vue';
import { useSettingsStore } from '../../stores/settings';
import { useToastStore } from '../../stores/toast';
import CookiesConfig from '../../components/authentication/CookiesConfig.vue';
import CredentialsConfig from '../../components/authentication/CredentialsConfig.vue';
import { StrongholdFields, useStrongholdStore } from '../../stores/stronghold';
import { useI18n } from 'vue-i18n';
import { AuthSettings } from '../../tauri/types/config.ts';

const { t } = useI18n();
const settingsStore = useSettingsStore();
const strongholdStore = useStrongholdStore();
const toastStore = useToastStore();

onMounted(async () => {
  try {
    if (!strongholdStore.status.unlocked) {
      return;
    }
    strongholdFields.value = await strongholdStore.getValues();
    strongholdSnapshot.value = JSON.stringify(strongholdFields.value);
  } catch (e) {
    console.error(e);
  }
});

const cookieFields = ref(JSON.parse(JSON.stringify(settingsStore.settings.auth)) as AuthSettings);
const strongholdFields = ref<StrongholdFields>({
  username: null, password: null, videoPassword: null, bearer: null, headers: null,
});
const strongholdSnapshot = ref('');

const isSaving = ref(false);
const hasCookieChanges = computed(() => {
  return JSON.stringify(cookieFields.value) !== JSON.stringify(settingsStore.settings.auth);
});
const hasStrongholdChanges = computed(() => {
  return JSON.stringify(strongholdFields.value) !== strongholdSnapshot.value && strongholdStore.status.unlocked === true;
});
const hasChanges = computed(() => {
  return hasCookieChanges.value || hasStrongholdChanges.value;
});

const save = async () => {
  isSaving.value = true;
  try {
    if (hasCookieChanges.value) {
      await settingsStore.patch({ auth: cookieFields.value });
    }
    if (hasStrongholdChanges.value && strongholdStore.status.unlocked) {
      await strongholdStore.setValues(strongholdFields.value);
      strongholdSnapshot.value = JSON.stringify(strongholdFields.value);
    }
    toastStore.showToast(t('auth.toasts.saved'), { style: 'success' });
  } catch (e) {
    toastStore.showToast(t('auth.toasts.error', { error: e }), { style: 'error' });
  } finally {
    isSaving.value = false;
  }
};
</script>
