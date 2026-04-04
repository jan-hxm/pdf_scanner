import { ref, computed } from "vue";

export const isSidebarOpen = ref(false);
export const activeSettingButton = ref("");
export const activeSettings = ref({});

export const highlightColor = computed(
  () => activeSettings.value.highlightColor
);
