<template>
  <div>
    <button @click="toggleTheme" class="btn-secondary sidebar-controls mb-8">
      {{ theme === "light" ? "🌙 Dunkel" : "☀️ Hell" }}
    </button>
  </div>
</template>

<script setup>
import { useTheme } from "../composables/useTheme";
const { theme, setTheme } = useTheme();

const toggleTheme = () => {
  setTheme(theme.value === "light" ? "dark" : "light");
};
</script>
