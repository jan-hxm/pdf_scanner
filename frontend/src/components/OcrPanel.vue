<template>
  <!--
    Deliberately a card rather than another `.notice` line.

    Every other message in this pane is something the user reads and moves past.
    This one is the only place where the search admits it did not look inside
    some of the files, and the only place that can fix it - as one more grey
    strip in a stack of grey strips it was read as trivia and skipped. So it is
    raised, outlined in the warning colour, and headed by the consequence
    ("nicht durchsucht") rather than by the cause ("kein Text").
  -->
  <section
    class="ocr-card"
    v-if="files.length > 0 || isRecognizing"
    :class="{ 'is-working': isRecognizing }"
  >
    <template v-if="isRecognizing">
      <h4 class="ocr-card-title">
        <span class="ocr-card-icon" aria-hidden="true">🔤</span>
        Text wird erkannt
      </h4>

      <div class="progress-section">
        <p class="progress-text">
          <span>
            Datei {{ ocrProgress.fileIndex || 1 }}/{{
              ocrProgress.fileTotal || files.length
            }}
          </span>
          <span class="progress-count" v-if="ocrProgress.pageTotal">
            Seite {{ ocrProgress.page }}/{{ ocrProgress.pageTotal }}
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
        <p class="ocr-hint">
          Die Ergebnisse oben bleiben stehen. Sobald die Erkennung fertig ist,
          sucht die App automatisch noch einmal.
        </p>
        <button class="btn-outline btn-sm cancel-btn" @click="cancelRecognition">
          Texterkennung abbrechen
        </button>
      </div>
    </template>

    <template v-else>
      <h4 class="ocr-card-title">
        <span class="ocr-card-icon" aria-hidden="true">🖼️</span>
        {{ files.length }}
        {{ files.length === 1 ? "Datei wurde" : "Dateien wurden" }} nicht
        durchsucht
      </h4>

      <!-- Cause and remedy in one breath, in words that do not assume the user
           knows what OCR is. -->
      <p class="ocr-card-body">
        {{ files.length === 1 ? "Diese Datei ist" : "Diese Dateien sind" }}
        eingescannt und
        {{ files.length === 1 ? "besteht" : "bestehen" }}
        nur aus Bildern &mdash; darin steht kein Text, den die Suche lesen kann.
        Die Texterkennung liest ihn einmalig aus, danach
        {{
          files.length === 1
            ? "ist die Datei ganz normal durchsuchbar"
            : "sind die Dateien ganz normal durchsuchbar"
        }}.
      </p>

      <ul class="ocr-file-list" :title="allNames">
        <li v-for="name in listedNames" :key="name" class="ocr-file">
          {{ name }}
        </li>
        <li v-if="hiddenCount > 0" class="ocr-file ocr-file-more">
          und {{ hiddenCount }} weitere
        </li>
      </ul>

      <p class="ocr-unavailable" v-if="!ocrStatus.available">
        <span aria-hidden="true">⚠️</span>
        <span>
          Texterkennung nicht verfügbar. {{ ocrStatus.reason }}
        </span>
      </p>

      <form class="ocr-form" v-else @submit.prevent="start">
        <!-- Tesseract cannot tell German from English by itself, so the question
             is unavoidable. Asking it as a question - rather than labelling a
             dropdown "Sprache" - is what makes the disabled button explicable. -->
        <label class="ocr-question" for="ocr-language">
          In welcher Sprache
          {{ files.length === 1 ? "ist das Dokument" : "sind die Dokumente" }}
          geschrieben?
        </label>
        <div class="ocr-controls">
          <select id="ocr-language" v-model="language" class="ocr-select">
            <option value="" disabled>Bitte wählen …</option>
            <option
              v-for="option in options"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
          <button
            type="submit"
            class="btn-primary recognize-btn"
            :disabled="!language"
          >
            Text erkennen
          </button>
        </div>

        <p class="ocr-hint">
          <template v-if="!language">
            Die Sprache lässt sich nicht automatisch bestimmen &mdash; bitte
            zuerst auswählen.
          </template>
          <template v-else>
            Dauert einige Sekunden pro Seite. Das Ergebnis wird dauerhaft
            gespeichert, jede weitere Suche findet den Text sofort.
          </template>
        </p>
      </form>
    </template>
  </section>
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

// Four fits the pane without scrolling; beyond that the count carries the
// information and the full list stays in the title attribute.
const MAX_LISTED = 4;
const language = ref("");

const names = computed(() => props.files.map((f) => f.fileName));
const allNames = computed(() => names.value.join(", "));
const listedNames = computed(() => names.value.slice(0, MAX_LISTED));
const hiddenCount = computed(() =>
  Math.max(0, names.value.length - MAX_LISTED)
);

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
