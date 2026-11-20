<template>
  <base-fieldset
      :legend="t('settings.notifications.legend')"
      :label="t('settings.notifications.legendLabel')"
      rows
  >
    <base-setting-row
        :label="t('settings.notifications.behavior.label')"
        :hint="t('settings.notifications.behavior.hint')"
        label-for="notification-behavior"
    >
      <select
          id="notification-behavior"
          v-model="settings.notifications.notificationBehavior"
          class="select w-56"
      >
        <option
            v-for="behavior in notificationBehaviors"
            :key="behavior.value"
            :value="behavior.value"
        >
          {{ behavior.label }}
        </option>
      </select>
    </base-setting-row>

    <div class="py-3.5 last:pb-0">
      <details
          :tabindex="canShowNotifications ? undefined : -1"
          class="collapse collapse-arrow rounded-lg border border-line bg-field"
          :class="{ 'opacity-50 cursor-not-allowed': !canShowNotifications }"
      >
        <summary class="collapse-title min-h-0 py-3 text-sm font-medium" :class="{ 'cursor-not-allowed': !canShowNotifications }">
          <h3>{{ t('settings.notifications.disabled.label') }}</h3>
          <span v-if="!canShowNotifications" class="sr-only">{{ t('settings.notifications.disabled.disabledScreenReader') }}</span>
        </summary>
        <div class="collapse-content grid grid-cols-1 gap-2 sm:grid-cols-2">
          <label
              v-for="(label, kind) in notificationKindLabels"
              :key="kind"
              class="flex cursor-pointer items-center gap-2.5 text-sm"
          >
            <input
                type="checkbox"
                class="toggle toggle-primary toggle-sm"
                :checked="!isDisabled(kind as NotificationKind)"
                @change="setDisabled(kind as NotificationKind, !($event.target as HTMLInputElement).checked)"
            />
            <span>{{ label }}</span>
          </label>
        </div>
      </details>
    </div>
  </base-fieldset>
</template>

<script setup lang="ts">
import BaseFieldset from '../base/BaseFieldset.vue';
import BaseSettingRow from '../base/BaseSettingRow.vue';
import { NotificationBehavior, Settings } from '../../tauri/types/config.ts';
import { useI18n } from 'vue-i18n';
import { computed, ComputedRef } from 'vue';
import { SelectOption } from '../../helpers/forms';
import { NotificationKind } from '../../tauri/types/app';

const i18n = useI18n();
const t = i18n.t;
const settings = defineModel<Settings>({ required: true });

const notificationBehaviors: ComputedRef<SelectOption[]> = computed(() => ([
  { value: NotificationBehavior.Never, label: t('settings.notifications.behavior.options.never') },
  { value: NotificationBehavior.OnBackground, label: t('settings.notifications.behavior.options.onBackground') },
  { value: NotificationBehavior.Always, label: t('settings.notifications.behavior.options.always') },
]));

const canShowNotifications = computed(() => settings.value.notifications.notificationBehavior !== NotificationBehavior.Never);

const notificationKindLabels = computed<Record<NotificationKind, string>>(() => {
  const rawValues = i18n.tm('settings.notifications.disabled.kinds');
  return rawValues as Record<NotificationKind, string>;
});

function isDisabled(kind: NotificationKind): boolean {
  return settings.value.notifications.disabledNotifications.includes(kind);
}

function setDisabled(kind: NotificationKind, disabled: boolean): void {
  const list = settings.value.notifications.disabledNotifications;
  const idx = list.indexOf(kind);

  if (disabled) {
    if (idx === -1) list.push(kind);
  } else {
    if (idx !== -1) list.splice(idx, 1);
  }
}

</script>
