<template>
  <div class="pdf-container">
    <div
      class="pdf-wrapper"
      v-if="renderCanvas"
      :class="isLoading ? 'invis-canvas' : ''"
    >
      <!--
        Highlight coordinates come out of usePdfViewer in canvas pixel space,
        so they must be measured from the canvas' own top-left corner. This
        wrapper is what makes that true: it is the positioned ancestor, and it
        hugs the canvas, so any padding or border on .pdf-wrapper cannot shift
        the boxes off the glyphs.
      -->
      <div class="pdf-page">
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
    </div>

    <!-- Nothing opened yet: say so, rather than leaving half the window blank. -->
    <div class="pdf-placeholder" v-if="!renderCanvas && !isLoading">
      <span class="pdf-placeholder-icon" aria-hidden="true">📄</span>
      <p>Noch kein Dokument geöffnet</p>
      <p>Einen Treffer in der Liste anklicken, um ihn hier anzuzeigen.</p>
    </div>

    <div :class="isLoading ? 'invis-canvas' : ''">
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <p v-if="viewerMessage" class="error">{{ viewerMessage }}</p>

      <div class="pdf-controls-container" v-if="renderCanvas">
        <div class="pdf-controls">
          <button
            class="btn-ghost btn-icon btn-sm"
            title="Vorherige Seite"
            aria-label="Vorherige Seite"
            :disabled="currentPage <= 1"
            @click="prevPage()"
          >
            <span aria-hidden="true">‹</span>
          </button>
          <p class="pdf-readout">Seite {{ currentPage }} / {{ totalPages }}</p>
          <button
            class="btn-ghost btn-icon btn-sm"
            title="Nächste Seite"
            aria-label="Nächste Seite"
            :disabled="currentPage >= totalPages"
            @click="nextPage()"
          >
            <span aria-hidden="true">›</span>
          </button>
        </div>

        <div class="pdf-controls">
          <button
            class="btn-ghost btn-icon btn-sm"
            title="Verkleinern"
            aria-label="Verkleinern"
            @click="scaleDown()"
          >
            <span aria-hidden="true">−</span>
          </button>
          <p class="pdf-readout">{{ Math.round(scale * 100) }} %</p>
          <button
            class="btn-ghost btn-icon btn-sm"
            title="Vergrößern"
            aria-label="Vergrößern"
            @click="scaleUp()"
          >
            <span aria-hidden="true">+</span>
          </button>
        </div>

        <div class="pdf-controls">
          <p class="pdf-controls-label">Marker</p>
          <ColorPicker></ColorPicker>
        </div>
      </div>
    </div>

    <p class="pdf-status" v-if="isLoading">PDF wird geladen …</p>
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
  viewerMessage,
} from "../composables/usePdfViewer.js";
import { highlightColor } from "../composables/useSettings.js";
import { errorMessage } from "../composables/useError.js";
import ColorPicker from "./ColorPicker.vue";
</script>
