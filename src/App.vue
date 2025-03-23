<template>
  <div>
    <HeaderComponent></HeaderComponent>
    <div class="flex-container">
      <SearchComponent></SearchComponent>
      <PdfViewer></PdfViewer>
      <span class="version">v1.02.64</span>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from "vue";
import PdfViewer from "./components/PdfViewer.vue";
import SearchComponent from "./components/SearchComponent.vue";
import HeaderComponent from "./components/HeaderComponent.vue";
import { activeSettings } from "./composables/useSettings";
import { searchHistory } from "./composables/useSearch";

onMounted(() => {});
ipcRenderer.invoke("settings:load").then((res) => {
  activeSettings.value = res;
  searchHistory.value = activeSettings.value.searchHistory;
});
</script>

<style>
@import "./assets/style/main.css";
</style>
