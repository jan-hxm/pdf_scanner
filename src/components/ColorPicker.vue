<template>
  <div class="color-picker-container">
    <button
      class="main-button"
      :class="{ active: isOpen }"
      :style="{ backgroundColor: currentColor }"
      @click="togglePicker"
    >
      <span class="color-name">{{ colorName }}</span>
    </button>

    <div class="color-options" :class="{ active: isOpen }">
      <div
        v-for="(color, index) in availableColors"
        :key="index"
        class="color-option"
        :style="{ backgroundColor: color }"
        @click="selectColor(color)"
      >
        <span class="color-name">{{ getColorName(color) }}</span>
      </div>
    </div>
  </div>

  <div class="overlay" :class="{ active: isOpen }" @click="closePicker"></div>
</template>

<script setup>
import { ref, computed } from "vue";
import { activeSettings } from "../composables/useSettings";
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
  ipcRenderer.invoke(
    "settings:save",
    JSON.parse(JSON.stringify(activeSettings.value))
  );
};

const getColorName = (color) => {
  return colorNames[color] || "Custom";
};
</script>

<style scoped>
.color-picker-container {
  position: relative;
  width: 48px;
  height: 48px;
}

.main-button {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  border: none;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transition: transform 0.3s ease, box-shadow 0.3s ease;
  position: relative;
  z-index: 10;
  display: flex;
  justify-content: center;
  align-items: center;
}

.main-button::after {
  content: "";
  width: 20px;
  height: 20px;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='white' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-size: contain;
  background-repeat: no-repeat;
  transition: transform 0.3s ease;
  filter: drop-shadow(0 1px 1px rgba(0, 0, 0, 0.3));
}

.main-button.active::after {
  transform: rotate(180deg);
}

.main-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.2);
}

.color-options {
  position: absolute;
  bottom: 70px;
  left: 50%;
  transform: translateX(-50%) scale(0.8);
  display: flex;
  visibility: hidden;
  opacity: 0;
  transition: transform 0.3s ease, opacity 0.3s ease, visibility 0.3s;
  z-index: 5;
}

.color-options.active {
  visibility: visible;
  opacity: 1;
  transform: translateX(-50%) scale(1);
}

.color-option {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  margin: 0 6px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  border: 2px solid white;
}

.color-option:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.color-name {
  position: absolute;
  top: -40px;
  left: 50%;
  transform: translateX(-50%);
  background-color: #333;
  color: white;
  padding: 4px 12px;
  border-radius: 16px;
  font-size: 14px;
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.main-button:hover .color-name,
.color-option:hover .color-name {
  opacity: 1;
}

/* Overlay when picker is active */
.overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: transparent;
  display: none;
  z-index: 2;
}

.overlay.active {
  display: block;
}

/* Color animation pulse for active button */
@keyframes pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(0, 0, 0, 0.2);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(0, 0, 0, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(0, 0, 0, 0);
  }
}

.main-button.active {
  animation: pulse 1.5s infinite;
}
</style>
