<template>
  <button
    @click="
      activeSettingButton == 'history'
        ? (activeSettingButton = '')
        : (activeSettingButton = 'history')
    "
    class="btn-secondary sidebar-controls mb-8"
  >
    📋Suchverlauf
  </button>
  <div v-if="activeSettingButton == 'history'">
    <h3>Suchverlauf</h3>
    <ul v-if="searchHistory.length > 0">
      <li
        v-for="entry in searchHistory"
        :key="entry.timestamp"
        @click="searchKeyword = entry.keyword"
      >
        <strong>{{ `${entry.keyword}` }}</strong> - 📆
        <small>{{ `${entry.timestamp}` }}</small>
      </li>
    </ul>
    <ul v-else>
      <li>Noch keine Einträge. 💡</li>
    </ul>
  </div>
</template>

<script setup>
import { searchKeyword, searchHistory } from "../composables/useSearch";
import { activeSettingButton } from "../composables/useSettings";
</script>
