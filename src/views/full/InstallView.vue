<template>
  <div class="mx-auto flex min-h-full w-full max-w-2xl flex-col gap-6 px-6 py-10">
    <div class="flex items-center gap-4">
      <img src="/icon.png" alt="" class="size-12 shrink-0 rounded-xl" />
      <div class="min-w-0">
        <h1 v-if="isInstalling" class="text-xl font-semibold">{{ t('install.title.installing') }}</h1>
        <h1 v-else-if="installPartialFail" class="text-xl font-semibold">{{ t('install.title.failed') }}</h1>
        <h1 v-else class="text-xl font-semibold">{{ t('install.title.installed') }}</h1>
        <p v-if="isInstalling" class="text-sm text-base-content/60">{{ t('install.subtitle.installing') }}</p>
        <p v-else-if="installPartialFail" class="text-sm text-base-content/60">{{ t('install.subtitle.failed') }}</p>
        <p v-else class="text-sm text-base-content/60">{{ t('install.subtitle.installed') }}</p>
      </div>
    </div>
    <div class="flex flex-col gap-3">
      <tool-card v-for="[name, tool] in Object.entries(tools)" :key="name" :tool="tool" :name="name"/>
    </div>
    <div class="flex justify-end">
      <base-button :disabled="isInstalling" class="gap-1.5" :class="{ 'btn-primary': !installPartialFail, 'btn-soft': installPartialFail }" to="/">
        <template v-if="isInstalling">
          {{ t('common.pleaseWait') }}
        </template>
        <template v-else-if="!installPartialFail">
          {{ t('common.parenthesis', { content: waitTime }) }} {{ t('common.continue') }} <arrow-right-icon class="size-4"/>
        </template>
        <template v-else>
          {{ t('common.continue') }} <arrow-right-icon class="size-4"/>
        </template>
      </base-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useBinariesStore } from '../../stores/binaries';
import { ArrowRightIcon } from '@heroicons/vue/24/solid';
import { computed, onMounted, ref } from 'vue';
import BaseButton from '../../components/base/BaseButton.vue';
import { useRouter } from 'vue-router';
import ToolCard from '../../components/tool-card/ToolCard.vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const binariesStore = useBinariesStore();
const router = useRouter();

const tools = computed(() => binariesStore.tools);
const isInstalling = ref(true);
const waitTime = ref(5);
const installPartialFail = ref(false);

onMounted(async () => {
  await ensureBinaries();
});

const ensureBinaries = async () => {
  try {
    await binariesStore.ensure();
    isInstalling.value = false;
    startCountdown();
  } catch (e) {
    installPartialFail.value = true;
    console.error(e);
  } finally {
    isInstalling.value = false;
  }
};

const startCountdown = () => {
  const ref = setInterval(() => {
    waitTime.value = waitTime.value - 1;
  }, 1000);
  setTimeout(() => {
    clearInterval(ref);
    void router.push('/');
  }, 5000);
};
</script>
