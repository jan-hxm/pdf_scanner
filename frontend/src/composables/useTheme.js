import { ref, watchEffect } from "vue";

const theme = ref(localStorage.getItem("theme") || "light");

export function useTheme() {
  const setTheme = (newTheme) => {
    theme.value = newTheme;
    document.documentElement.setAttribute("data-theme", newTheme);
    localStorage.setItem("theme", newTheme);
  };

  // Beim Laden das Theme setzen
  watchEffect(() => {
    document.documentElement.setAttribute("data-theme", theme.value);
  });

  return { theme, setTheme };
}
