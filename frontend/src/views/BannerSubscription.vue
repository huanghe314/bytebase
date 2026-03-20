<template>
  <!-- Modified: License banner disabled for local dev -->
</template>

<script lang="ts" setup>
import { storeToRefs } from "pinia";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { SETTING_ROUTE_WORKSPACE_SUBSCRIPTION } from "@/router/dashboard/workspaceSetting";
import { LICENSE_EXPIRATION_THRESHOLD, useSubscriptionV1Store } from "@/store";
import { PlanType } from "@/types/proto-es/v1/subscription_service_pb";

const { t } = useI18n();
const subscriptionStore = useSubscriptionV1Store();

const currentPlanText = computed((): string => {
  const plan = subscriptionStore.currentPlan;
  switch (plan) {
    case PlanType.TEAM:
      return t("subscription.plan.team.title");
    case PlanType.ENTERPRISE:
      return t("subscription.plan.enterprise.title");
    default:
      return t("subscription.plan.free.title");
  }
});

const content = computed(() => {
  // Modified: Suppress all license expiration warnings for local dev
  return "";
});

const { expireAt, isExpired, isTrialing, daysBeforeExpire, currentPlan } =
  storeToRefs(subscriptionStore);
</script>
