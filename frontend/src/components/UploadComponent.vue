<template>
  <div class="upload-component">
    <button
      type="button"
      class="btn-outline btn-block btn-start upload-btn"
      :aria-expanded="uploadActive"
      @click="uploadActive = !uploadActive"
    >
      <span aria-hidden="true">📄</span>
      <span class="btn-label">Neue PDF-Datei hinzufügen</span>
      <span class="disclosure" aria-hidden="true">▾</span>
    </button>
    <transition name="top">
      <div class="upload-container" v-if="uploadActive">
        <!--
          `--wails-drop-target: drop` marks this element as a native Wails drop
          target. Wails then reports the real absolute paths through the
          "files-dropped" event — WebView2 has no File.path to read.
        -->
        <div
          class="drop-area"
          style="--wails-drop-target: drop"
          @click="selectFiles"
        >
          <h2>Anklicken oder PDF-Dateien hierher ziehen</h2>
          <p class="drop-hint">Nur .pdf</p>
        </div>
      </div>
    </transition>

    <p v-if="errorMessage" class="error upload-feedback">{{ errorMessage }}</p>
    <p v-if="successMessage" class="success-message upload-feedback">
      {{ successMessage }}
    </p>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import {
  SelectFiles,
  CopyFileToPDFsFolder,
  GetPDFsFolder,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

const errorMessage = ref("");
const successMessage = ref("");
const uploadActive = ref(false);
const uploadFolder = ref("");

let clearTimer = null;

/** Shows a message and clears it again, so old feedback never lingers. */
const report = ({ success = "", error = "" }) => {
  successMessage.value = success;
  errorMessage.value = error;

  clearTimeout(clearTimer);
  clearTimer = setTimeout(() => {
    successMessage.value = "";
    errorMessage.value = "";
  }, 5000);
};

const importFiles = async (filePaths) => {
  const pdfs = filePaths.filter((p) => p.toLowerCase().endsWith(".pdf"));
  const skipped = filePaths.length - pdfs.length;

  if (pdfs.length === 0) {
    report({ error: "Bitte nur PDF-Dateien hinzufügen!" });
    return;
  }

  const copied = [];
  for (const filePath of pdfs) {
    try {
      copied.push(await CopyFileToPDFsFolder(filePath));
    } catch (err) {
      report({ error: `Fehler bei ${filePath}: ${err.message || err}` });
      return;
    }
  }

  const skippedNote = skipped > 0 ? ` (${skipped} nicht-PDF übersprungen)` : "";
  report({
    success: `${copied.join(", ")} nach ${uploadFolder.value} kopiert.${skippedNote}`,
  });
};

const selectFiles = async () => {
  try {
    const filePaths = await SelectFiles();
    if (filePaths && filePaths.length > 0) await importFiles(filePaths);
  } catch (err) {
    report({ error: `Fehler bei der Dateiauswahl: ${err.message || err}` });
  }
};

const onFilesDropped = (paths) => {
  if (!uploadActive.value || !paths || paths.length === 0) return;
  importFiles(paths);
};

onMounted(() => {
  EventsOn("files-dropped", onFilesDropped);
  GetPDFsFolder().then((res) => {
    uploadFolder.value = res;
  });
});

onUnmounted(() => {
  clearTimeout(clearTimer);
  EventsOff("files-dropped");
});
</script>
