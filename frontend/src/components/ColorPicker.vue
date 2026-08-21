<template>
  <div class="color-picker-container">
    <button
      class="main-button"
      type="button"
      :class="{ active: isOpen }"
      :aria-expanded="isOpen"
      :aria-label="`Markerfarbe: ${colorName}`"
      :style="{ backgroundColor: currentColor }"
      @click="togglePicker"
    >
      <span class="color-name">{{ colorName }}</span>
    </button>

    <div class="color-options" :class="{ active: isOpen }">
      <button
        v-for="(color, index) in availableColors"
        :key="index"
        type="button"
        class="color-option"
        :style="{ backgroundColor: color }"
        :aria-label="`Markerfarbe ${getColorName(color)}`"
        :tabindex="isOpen ? 0 : -1"
        @click="selectColor(color)"
      >
        <span class="color-name">{{ getColorName(color) }}</span>
      </button>
    </div>
  </div>

  <div class="overlay" :class="{ active: isOpen }" @click="closePicker"></div>
</template>

<script setup>
import { ref, computed } from "vue";
import { activeSettings } from "../composables/useSettings";
import { SaveSettings } from "../../wailsjs/go/main/App";

const isOpen = ref(false);
const currentColor = computed(() => activeSettings.value.highlightColor);
const allColors = ref(["#4A90E2", "#50E3C2", "#F5A623", "#E74C3C", "#9B59B6"]);
const colorNames = {
  "#4A90E2": "Blue",
  "#50E3C2": "Turquoise",
  "#F5A623": "Orange",
  "#E74C3C": "Red",
  "#9B59B6": "Purple",
};

const colorName = computed(() => {
  return colorNames[currentColor.value] || "Custom";
});

const availableColors = computed(() => {
  return allColors.value.filter((color) => color !== currentColor.value);
});

const togglePicker = () => {
  isOpen.value = !isOpen.value;
};

const closePicker = () => {
  isOpen.value = false;
};

const selectColor = (color) => {
  activeSettings.value.highlightColor = color;
  isOpen.value = false;
  SaveSettings(JSON.parse(JSON.stringify(activeSettings.value)));
};

const getColorName = (color) => {
  return colorNames[color] || "Custom";
};
</script>

<style scoped>
/* Sits in the viewer's control bar, so it matches the small control height
   there rather than being a 48px feature button. */
.color-picker-container {
  position: relative;
  width: var(--control-height-sm);
  height: var(--control-height-sm);
}

.main-button {
  width: var(--control-height-sm);
  height: var(--control-height-sm);
  border-radius: 50%;
  border: 2px solid var(--surface);
  cursor: pointer;
  box-shadow: 0 0 0 1px var(--border), var(--shadow-xs);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
  position: relative;
  z-index: 10;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0;
}

.main-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 0 0 1px var(--border-strong), var(--shadow-sm);
}

.main-button:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.color-options {
  position: absolute;
  bottom: calc(100% + var(--space-3));
  left: 50%;
  transform: translateX(-50%) scale(0.9);
  display: flex;
  padding: var(--space-2);
  gap: var(--space-2);
  background: var(--surface-raised);
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow-lg);
  visibility: hidden;
  opacity: 0;
  transition: transform var(--transition-fast), opacity var(--transition-fast),
    visibility var(--transition-fast);
  z-index: 5;
}

.color-options.active {
  visibility: visible;
  opacity: 1;
  transform: translateX(-50%) scale(1);
}

.color-option {
  position: relative;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid var(--surface-raised);
  box-shadow: 0 0 0 1px var(--border);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}

.color-option:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.color-option:hover,
.color-option:focus-visible {
  transform: translateY(-2px);
  box-shadow: 0 0 0 1px var(--border-strong), var(--shadow-sm);
}

/* Name tooltip above the swatch, shown on hover only. */
.color-name {
  position: absolute;
  bottom: calc(100% + var(--space-2));
  left: 50%;
  transform: translateX(-50%);
  background-color: var(--tooltip-bg);
  color: var(--tooltip-fg);
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  white-space: nowrap;
  opacity: 0;
  transition: opacity var(--transition-fast);
  pointer-events: none;
}

.main-button:hover .color-name,
.main-button:focus-visible .color-name,
.color-option:hover .color-name,
.color-option:focus-visible .color-name {
  opacity: 1;
}

.overlay {
  position: fixed;
  inset: 0;
  background: transparent;
  display: none;
  z-index: 2;
}

.overlay.active {
  display: block;
}
</style>
