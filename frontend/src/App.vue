<template>
  <div class="app-shell">
    <HeaderComponent></HeaderComponent>
    <main class="flex-container app-main">
      <SearchComponent></SearchComponent>
      <PdfViewer></PdfViewer>
    </main>
    <span class="version">v{{ version }}</span>
  </div>
</template>

<script setup>
import { onMounted } from "vue";
import PdfViewer from "./components/PdfViewer.vue";
import SearchComponent from "./components/SearchComponent.vue";
import HeaderComponent from "./components/HeaderComponent.vue";
import { activeSettings } from "./composables/useSettings";
import { searchHistory } from "./composables/useSearch";
import { LoadSettings } from "../wailsjs/go/main/App";

const version = __APP_VERSION__;

onMounted(() => {
  LoadSettings().then((res) => {
    activeSettings.value = res;
    searchHistory.value = activeSettings.value.searchHistory || [];
  });
});
</script>

<style>
@import "./assets/style/main.css";
</style>
