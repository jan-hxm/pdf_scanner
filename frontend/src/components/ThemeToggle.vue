<template>
  <button
    type="button"
    class="btn-outline btn-block btn-start"
    :aria-pressed="theme === 'dark'"
    @click="toggleTheme"
  >
    <span aria-hidden="true">{{ theme === "light" ? "🌙" : "☀️" }}</span>
    {{ theme === "light" ? "Dunkles Design" : "Helles Design" }}
  </button>
</template>

<script setup>
import { useTheme } from "../composables/useTheme";
const { theme, setTheme } = useTheme();

const toggleTheme = () => {
  setTheme(theme.value === "light" ? "dark" : "light");
};
</script>
