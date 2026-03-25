<template>
  <!-- Upgrade button disabled -->
  <WeChatQRModal
    v-if="state.showQRCodeModal"
    :title="$t('subscription.request-with-qr')"
    @close="state.showQRCodeModal = false"
  />
</template>

<script lang="ts" setup>
import { reactive } from "vue";
import WeChatQRModal from "./WeChatQRModal.vue";

interface LocalState {
  showQRCodeModal: boolean;
}

const state = reactive<LocalState>({
  showQRCodeModal: false,
});
</script>
