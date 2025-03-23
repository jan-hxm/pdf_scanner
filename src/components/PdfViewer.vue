<template>
  <div class="pdf-container">
    <div
      class="pdf-wrapper"
      v-if="renderCanvas"
      :class="isLoading ? 'invis-canvas' : ''"
    >
      <canvas ref="canvasRef"></canvas>
      <div
        v-for="(hl, index) in highlights"
        :key="index"
        class="highlight"
        :style="{
          left: `${hl.x}px`,
          top: `${hl.y}px`,
          width: `${hl.width}px`,
          height: `${hl.height}px`,
          backgroundColor: highlightColor,
        }"
      ></div>
    </div>
    <div :class="isLoading ? 'invis-canvas' : ''">
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <div class="pdf-controls-container" v-if="renderCanvas">
        <div class="pdf-controls">
          <button class="btn-primary" @click="prevPage()">{{ "<" }}</button>
          <p>Seite {{ currentPage }} / {{ totalPages }}</p>
          <button class="btn-primary" @click="nextPage()">></button>
        </div>
        <div class="pdf-controls">
          <button class="btn-primary" @click="scaleDown()">-</button>
          <p>🔎Zoom {{ scale.toFixed(1) }}</p>
          <button class="btn-primary" @click="scaleUp()">+</button>
        </div>
        <div class="pdf-controls">
          <p>Marker:</p>
          <ColorPicker></ColorPicker>
        </div>
      </div>
    </div>
    <p v-if="isLoading">📄 lade PDF...</p>
  </div>
</template>

<script setup>
import {
  canvasRef,
  highlights,
  isLoading,
  currentPage,
  totalPages,
  nextPage,
  prevPage,
  renderCanvas,
  scaleUp,
  scaleDown,
  scale,
} from "../composables/usePdfViewer.js";
import { highlightColor } from "../composables/useSettings.js";
import { errorMessage } from "../composables/useError.js";
import ColorPicker from "./ColorPicker.vue";
</script>
