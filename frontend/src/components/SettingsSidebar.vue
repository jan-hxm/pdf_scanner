<template>
  <button
    class="btn-ghost btn-icon settings-toggle"
    type="button"
    :aria-expanded="isSidebarOpen"
    aria-controls="settings-panel"
    :aria-label="isSidebarOpen ? 'Einstellungen schließen' : 'Einstellungen öffnen'"
    :title="isSidebarOpen ? 'Einstellungen schließen' : 'Einstellungen öffnen'"
    @click="isSidebarOpen = !isSidebarOpen"
  >
    <span aria-hidden="true">{{ isSidebarOpen ? "✕" : "⚙️" }}</span>
  </button>

  <!-- Clicking outside closes the panel; without it the only way back out is
       the same small icon the user just left. -->
  <div
    v-if="isSidebarOpen"
    class="sidebar-scrim"
    @click="isSidebarOpen = false"
  ></div>

  <transition name="slide">
    <aside class="sidebar" id="settings-panel" v-if="isSidebarOpen">
      <h2 class="sidebar-title">Einstellungen</h2>

      <div class="sidebar-section">
        <p class="sidebar-label">Darstellung</p>
        <ThemeToggle></ThemeToggle>
      </div>

      <div class="sidebar-section">
        <PdfsFolder></PdfsFolder>
      </div>

      <div class="sidebar-section">
        <OcrCache></OcrCache>
      </div>

      <div class="sidebar-section">
        <SearchHistory></SearchHistory>
      </div>
    </aside>
  </transition>
</template>

<script setup>
import { isSidebarOpen } from "../composables/useSettings";
import SearchHistory from "./SearchHistory.vue";
import ThemeToggle from "./ThemeToggle.vue";
import PdfsFolder from "./PdfsFolder.vue";
import OcrCache from "./OcrCache.vue";
</script>
