<template>
  <p class="sidebar-label">Suchverlauf</p>

  <button
    type="button"
    class="btn-outline btn-block btn-start"
    :aria-expanded="isOpen"
    aria-controls="search-history-list"
    @click="activeSettingButton = isOpen ? '' : 'history'"
  >
    <span aria-hidden="true">📋</span>
    <span class="btn-label">Letzte Suchen</span>
    <span class="disclosure" aria-hidden="true">▾</span>
  </button>

  <div v-if="isOpen" id="search-history-list">
    <ul class="history-list" v-if="searchHistory.length > 0">
      <li v-for="entry in searchHistory" :key="entry.timestamp">
        <button
          type="button"
          class="history-item"
          @click="searchKeyword = entry.keyword"
        >
          <span class="history-keyword">{{ entry.keyword }}</span>
          <span class="history-time">{{ entry.timestamp }}</span>
        </button>
      </li>
    </ul>
    <p class="history-empty" v-else>Noch keine Einträge.</p>
  </div>
</template>

<script setup>
import { computed } from "vue";
import { searchKeyword, searchHistory } from "../composables/useSearch";
import { activeSettingButton } from "../composables/useSettings";

const isOpen = computed(() => activeSettingButton.value === "history");
</script>
