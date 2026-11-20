<template>
  <component
      :is="hasDetails ? 'details' : 'section'"
      class="collapse rounded-box border border-line bg-base-100"
      :class="{ 'collapse-arrow': hasDetails, 'cursor-auto': !hasDetails }"
  >
    <component
        :is="hasDetails ? 'summary' : 'div'"
        class="collapse-title text-sm flex flex-row items-center gap-2"
        :class="{ 'cursor-auto': !hasDetails }"
    >
      <span
          v-if="diagnostic.component"
          class="badge badge-soft shrink-0 whitespace-nowrap"
          :class="diagnosticBadgeClass"
      >
        {{ diagnostic.component }}
      </span>
      <span
          v-if="relatedFatal && diagnostic.level ===  'error'"
          class="badge badge-soft badge-error shrink-0 whitespace-nowrap"
      >
        <exclamation-circle-icon class="w-5 h-5"/>
        {{ t('media.view.logs.failure') }}
      </span>
      <span v-html="diagnosticDisplay.message"/>
    </component>
    <div v-if="hasDetails" class="collapse-content text-sm flex flex-col gap-2">
      <p
          class="max-h-44 overflow-y-auto rounded-lg bg-field p-4 font-mono text-xs leading-5 text-wrap"
          v-if="diagnostic.raw"
          v-html="diagnosticDisplay.raw"
      />
      <button @click="report" v-if="isReportable" :disabled="hasReported" class="btn btn-soft btn-sm self-start">
        <template v-if="isReporting">
          <span class="sr-only">{{ t('common.loading') }}</span>
          <span class="loading loading-spinner loading-sm"></span>
        </template>
        <template v-else>
          <span>{{ t('media.steps.error.report') }}</span>
        </template>
      </button>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, PropType, ref } from 'vue';
import { ExclamationCircleIcon } from '@heroicons/vue/24/solid';
import { useI18n } from 'vue-i18n';
import { useDiagnostic } from '../../composables/useDiagnostic.ts';

const { t } = useI18n();

const { diagnostic } = defineProps({
  diagnostic: {
    type: Object as PropType<MediaDiagnostic>,
    required: true,
  },
});

const diagnosticBadgeClass = computed(() => {
  return {
    'badge-warning': diagnostic.level === 'warning',
    'badge-error': diagnostic.level === 'error',
  };
});

const hasDetails = computed(() => diagnostic.raw != '');

const {
  diagnosticDisplay,
  report,
  isReportable,
  isReporting,
  hasReported,
  relatedFatal,
} = useDiagnostic(ref(diagnostic));

</script>
