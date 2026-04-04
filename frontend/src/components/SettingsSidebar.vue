<template>
  <div>
    <button class="toggle-btn" @click="isSidebarOpen = !isSidebarOpen">
      {{ isSidebarOpen ? "❌" : "⚙️" }}
    </button>
    <transition name="slide">
      <div class="sidebar" v-if="isSidebarOpen">
        <h3>Einstellungen</h3>
        <ThemeToggle></ThemeToggle>
        <SearchHistory></SearchHistory>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { isSidebarOpen } from "../composables/useSettings";
import SearchHistory from "./SearchHistory.vue";
import ThemeToggle from "./ThemeToggle.vue";
</script>
