import { ref } from "vue";
import { activeSettings } from "./useSettings";

export const searchKeyword = ref("");
export const searchHistory = ref([]);

function formatTimestamp() {
  const now = new Date();
  const day = now.getDate();
  const month = now.getMonth() + 1;
  const hours = now.getHours();
  const minutes = now.getMinutes().toString().padStart(2, "0");

  return `${day}.${month}. ${hours}:${minutes}`;
}

export function addToSearchHistory(keyword) {
  if (!keyword.trim()) return;

  const timestamp = formatTimestamp();
  searchHistory.value.unshift({ timestamp, keyword });

  if (searchHistory.value.length > 23) {
    searchHistory.value.pop();
  }
  activeSettings.value.searchHistory = searchHistory.value;

  ipcRenderer.invoke(
    "settings:save",
    JSON.parse(JSON.stringify(activeSettings.value))
  );
}
