<template>
  <div class="flex h-full min-h-0 w-full">
    <the-sidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <!--
        The link bar and the action bar belong to the queue, so they are only
        shown there. v-show keeps them mounted: the link bar keeps watching the
        clipboard and the action bar keeps the global format choice applied
        while another page is open.
      -->
      <the-header v-show="isHome" />
      <main class="relative min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto bg-base-200">
        <router-view :key="$route.fullPath" />
      </main>
      <the-footer v-show="isHome" />
    </div>
  </div>
</template>
<script setup lang="ts">
import TheFooter from '../components/TheFooter.vue';
import TheHeader from '../components/TheHeader.vue';
import TheSidebar from '../components/TheSidebar.vue';
import { computed, ref, watchEffect } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useMediaGroupStore } from '../stores/media/group.ts';

const route = useRoute();
const router = useRouter();
const groupStore = useMediaGroupStore();
const isMissingGroupRedirecting = ref(false);

const isHome = computed(() => route.name === 'home');

watchEffect(() => {
  const requiresGroup = route.matched.some(record => record.meta.requiresGroup);
  if (!requiresGroup) {
    return;
  }

  const rawGroupId = route.params.groupId;
  const groupId = Array.isArray(rawGroupId) ? rawGroupId[0] : rawGroupId;
  if (!groupId || !groupStore.findGroupById(groupId)) {
    if (isMissingGroupRedirecting.value) return;
    isMissingGroupRedirecting.value = true;
    void router.replace({ name: 'home' }).finally(() => {
      isMissingGroupRedirecting.value = false;
    });
  }
});
</script>
<style>
#app {
  display: flex;
  height: 100vh;
  flex-direction: column;
}
</style>
