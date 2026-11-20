<template>
  <div class="flex min-w-0 flex-col gap-2">
    <base-progress :id="`${group.id}-progress`">
      {{ t('media.steps.fetch.progressLabel') }}
    </base-progress>
  </div>
</template>

<script setup lang="ts">

import { PropType } from 'vue';
import BaseProgress from '../../base/BaseProgress.vue';
import { Group } from '../../../tauri/types/group';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const { group } = defineProps({
  group: {
    type: Object as PropType<Group>,
    required: true,
  },
});

</script>
