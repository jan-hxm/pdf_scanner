<template>
  <div class="ocr-panel" v-if="files.length > 0 || isRecognizing">
    <p
      class="notice notice-warning"
      v-if="files.length > 0"
      :title="allNames"
    >
      <span aria-hidden="true">🖼️</span>
      <span>
        {{ files.length }}
        {{ files.length === 1 ? "Datei enthält" : "Dateien enthalten" }}
        keinen durchsuchbaren Text (vermutlich
        {{ files.length === 1 ? "ein Scan" : "Scans" }}):
        {{ shortNames }}
      </span>
    </p>

    <div class="progress-section" v-if="isRecognizing">
      <p class="progress-text">
        <span>Texterkennung</span>
        <span class="progress-count">
          Datei {{ ocrProgress.fileIndex || 1 }}/{{
            ocrProgress.fileTotal || files.length
          }}
          <template v-if="ocrProgress.pageTotal">
            · Seite {{ ocrProgress.page }}/{{ ocrProgress.pageTotal }}
          </template>
        </span>
      </p>
      <progress
        max="100"
        :value="ocrProgress.progress"
        class="progress-bar"
      ></progress>
      <p class="ocr-current" v-if="ocrProgress.fileName">
        {{ ocrProgress.fileName }}
      </p>
      <button
        class="btn-outline btn-sm cancel-btn"
        @click="cancelRecognition"
      >
        Texterkennung abbrechen
      </button>
    </div>

    <p class="notice notice-empty" v-else-if="!ocrStatus.available">
      <span aria-hidden="true">ℹ️</span>
      <span>
        {{ ocrStatus.reason }}
        Für die Texterkennung wird Tesseract OCR benötigt — mit deutschen
        Sprachdaten installieren, danach die App neu starten.
      </span>
    </p>

    <form class="ocr-controls" v-else @submit.prevent="start">
      <label class="ocr-language">
        <span class="ocr-label">Sprache</span>
        <select v-model="language" class="ocr-select">
          <option value="" disabled>Bitte wählen</option>
          <option
            v-for="option in options"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
      </label>
      <button
        type="submit"
        class="btn-primary btn-sm recognize-btn"
        :disabled="!language"
      >
        Text erkennen
      </button>
    </form>

    <p class="ocr-hint" v-if="!isRecognizing && ocrStatus.available">
      Dauert einige Sekunden pro Seite. Das Ergebnis wird gespeichert — jede
      weitere Suche findet den Text sofort.
    </p>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  ocrStatus,
  isRecognizing,
  ocrProgress,
  loadOcrStatus,
  recognizeFiles,
  cancelRecognition,
  applyOcrProgress,
  languageOptions,
} from "../composables/useOcr";
import { throwError } from "../composables/useError";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

const props = defineProps({
  files: {
    type: Array,
    default: () => [],
  },
});

const emit = defineEmits(["completed"]);

const MAX_LISTED = 3;
const language = ref("");

const names = computed(() => props.files.map((f) => f.fileName));
const allNames = computed(() => names.value.join(", "));
const shortNames = computed(() => {
  if (names.value.length <= MAX_LISTED) return names.value.join(", ");
  const rest = names.value.length - MAX_LISTED;
  return `${names.value.slice(0, MAX_LISTED).join(", ")} und ${rest} weitere`;
});

const options = computed(() => languageOptions(ocrStatus.value.languages));

// The language is asked for per run rather than remembered: Tesseract cannot
// tell German from English by itself (its auto-detection covers script, and
// both are Latin), and a remembered answer would silently be the wrong one for
// the next document. Clearing it whenever the file list changes is what makes
// the question get asked again.
watch(
  () => props.files,
  () => {
    language.value = "";
  }
);

const start = async () => {
  if (!language.value) return;

  const paths = props.files.map((f) => f.file);
  try {
    const report = await recognizeFiles(paths, language.value);
    // Even a cancelled run may have finished whole files before it stopped, so
    // the search is re-run either way — those files are searchable now.
    emit("completed", report);
  } catch (error) {
    throwError("Texterkennung fehlgeschlagen", error);
  }
};

const onOcrProgress = (data) => {
  if (isRecognizing.value) applyOcrProgress(data);
};

onMounted(() => {
  loadOcrStatus();
  EventsOn("ocr-progress", onOcrProgress);
});
onUnmounted(() => EventsOff("ocr-progress"));
</script>
