<template>
  <div class="-mr-1 flex shrink-0 items-center gap-0.5">
    <media-card-action-item
        v-if="canSetPreferences"
        :to="{ name: 'preferences.quality', params: { groupId: group.id } }"
        :label="t('media.card.actions.preferences')"
        :icon="AdjustmentsHorizontalIcon"
    />
    <media-card-action-item
        v-if="canViewInfo"
        :to="{ name: 'group.metadata', params: { groupId: group.id } }"
        :label="t('media.card.actions.metadata')"
        :icon="InformationCircleIcon"
    />
    <media-card-action-item
        @click="openExternalUrl"
        :disabled="!isValidExternalUrl"
        :label="t('media.card.actions.externalUrl')"
        :icon="ArrowTopRightOnSquareIcon"
    />
    <media-card-action-item
        @click="removeItem"
        :label="t('media.card.actions.remove')"
        :icon="XMarkIcon"
    />
  </div>
</template>
<script setup lang="ts">
import {
  AdjustmentsHorizontalIcon,
  ArrowTopRightOnSquareIcon,
  InformationCircleIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline';
import MediaCardActionItem from './MediaCardActionItem.vue';
import { computed, PropType } from 'vue';
import { useOpener } from '../../../composables/useOpener';
import { MediaState, useMediaStateStore } from '../../../stores/media/state';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const { openUrl } = useOpener();

const emit = defineEmits<{
  (e: 'remove'): void;
}>();

const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

const stateStore = useMediaStateStore();
const groupState = computed(() => stateStore.getGroupState(group.id));

const canViewInfo = computed(() => groupState.value !== MediaState.fetching && groupState.value !== MediaState.playlistSelection);
const canSetPreferences = computed(() => groupState.value === MediaState.configure);

const isValidExternalUrl = computed(() => {
  // Matches the requirement in 'src-tauri/capabilities/default.json' at 'permissions[4].allow[0].url'.
  const isAllowed = group.url.startsWith('https');
  if (!isAllowed) return false;
  try {
    new URL(group.url);
    return true;
  } catch {
    return false;
  }
});

const removeItem = (): void => {
  emit('remove');
};

const openExternalUrl = (): void => {
  void openUrl(group.url ?? '');
};
</script>
