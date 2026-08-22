<template>
  <div class="ocr-cache">
    <p class="sidebar-label">Texterkennung</p>

    <p class="cache-summary">
      <template v-if="cacheStats.entries > 0">
        {{ cacheStats.entries }}
        {{ cacheStats.entries === 1 ? "Dokument" : "Dokumente" }} gespeichert
        <span class="cache-size">{{ formatBytes(cacheStats.bytes) }}</span>
      </template>
      <template v-else>Noch nichts gespeichert.</template>
    </p>

    <!--
      Two steps, not one. Clearing is cheap to undo in the sense that the text
      can always be recognised again - and expensive in the sense that doing so
      costs minutes per document. That is exactly the shape of action that wants
      a second click and not a modal.
    -->
    <template v-if="confirming">
      <p class="sidebar-hint cache-warning">
        {{ cacheStats.entries }}
        {{ cacheStats.entries === 1 ? "Dokument" : "Dokumente" }} verwerfen? Die
        Texterkennung muss danach neu laufen.
      </p>
      <div class="cache-confirm">
        <button
          type="button"
          class="btn-outline btn-sm cache-danger"
          :disabled="busy"
          @click="confirm"
        >
          Ja, leeren
        </button>
        <button
          type="button"
          class="btn-ghost btn-sm"
          :disabled="busy"
          @click="confirming = false"
        >
          Abbrechen
        </button>
      </div>
    </template>

    <button
      v-else
      type="button"
      class="btn-outline btn-block btn-start btn-sm"
      :disabled="cacheStats.entries === 0 || isRecognizing"
      :title="disabledReason"
      @click="confirming = true"
    >
      <span aria-hidden="true">🗑️</span>Zwischenspeicher leeren
    </button>

    <p class="sidebar-hint">
      Text aus eingescannten PDFs wird dauerhaft gespeichert, damit jede weitere
      Suche ihn sofort findet. Leeren hilft, wenn ein Dokument in der falschen
      Sprache erkannt wurde.
    </p>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from "vue";
import {
  cacheStats,
  isRecognizing,
  loadCacheStats,
  clearCache,
  formatBytes,
} from "../composables/useOcr";
import { throwError } from "../composables/useError";

const confirming = ref(false);
const busy = ref(false);

// A disabled control with no explanation is a bug report waiting to happen, and
// the two reasons this one disables are not interchangeable.
const disabledReason = computed(() => {
  if (isRecognizing.value) return "Läuft gerade - bitte abwarten.";
  if (cacheStats.value.entries === 0) return "Es ist nichts gespeichert.";
  return "";
});

const confirm = async () => {
  busy.value = true;
  try {
    await clearCache();
    confirming.value = false;
  } catch (error) {
    throwError("Zwischenspeicher konnte nicht geleert werden", error);
  } finally {
    busy.value = false;
  }
};

// The sidebar is v-if'd, so this remounts every time it is opened — which is
// the only moment the number is looked at. Nothing here polls.
onMounted(loadCacheStats);
</script>
