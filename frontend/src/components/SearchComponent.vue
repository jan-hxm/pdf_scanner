<template>
  <div class="search-sidebar">
    <UploadComponent></UploadComponent>
    <div class="hr"></div>

    <div class="search-section">
      <h4><label for="search-keyword">Schlüsselwort</label></h4>
      <form @submit.prevent="startSearch" class="search-form">
        <input
          id="search-keyword"
          v-model="searchKeyword"
          class="input-field"
          type="search"
          placeholder="Begriff eingeben"
        />
        <button
          type="submit"
          class="btn-primary search-btn"
          :disabled="isSearching"
        >
          <span aria-hidden="true">🔍</span>Suchen
        </button>
      </form>
      <p class="error" v-if="errorMessage">{{ errorMessage }}</p>
    </div>

    <div class="progress-section" v-if="isSearching">
      <p class="progress-text">Durchsucht: {{ progress }}%</p>
      <progress max="100" :value="progress" class="progress-bar"></progress>
      <button class="btn-outline btn-sm cancel-btn" @click="cancelSearch">
        Suche abbrechen
      </button>
    </div>

    <p class="notice notice-warning" v-if="failures.length > 0">
      <span aria-hidden="true">⚠️</span>
      <span>
        {{ failures.length }}
        {{ failures.length === 1 ? "Datei konnte" : "Dateien konnten" }} nicht
        gelesen werden: {{ failures.map((f) => f.fileName).join(", ") }}
      </span>
    </p>

    <div class="result-heading-container" v-if="groupedResults.length > 0">
      <h3>{{ groupedResults.length }} Resultate</h3>
      <p>Treffer anklicken, um das Dokument zu untersuchen.</p>
    </div>

    <div class="results-section" v-if="groupedResults.length > 0">
      <ul class="results-list">
        <li
          v-for="group in groupedResults"
          :key="group.filePath"
          class="result-item"
        >
          <button
            type="button"
            class="file-row"
            :aria-expanded="!!expandedFiles[group.filePath]"
            @click="
              expandedFiles[group.filePath] = !expandedFiles[group.filePath]
            "
          >
            <span class="hit-count">{{ group.pages.length }}</span>
            <span class="file-name" :title="group.fileName">
              {{ group.fileName }}
            </span>
            <span class="disclosure" aria-hidden="true">▾</span>
          </button>

          <ul v-if="expandedFiles[group.filePath]" class="page-list">
            <li
              v-for="page in group.pages"
              :key="`${group.filePath}-${page.page}-${page.foundWord}`"
              class="page-item"
            >
              <div class="page-info-container">
                <button
                  type="button"
                  class="page-info"
                  @click="
                    showPdfFromUrl(group.filePath, page.page, page.foundWord)
                  "
                >
                  <span class="found-word">{{ page.foundWord }}</span>
                  <span class="page-meta">
                    Seite {{ page.page }} ·
                    <span :title="`Übereinstimmung: ${page.confidence}%`">
                      {{
                        page.confidence >= 100 ? "exakt" : `~${page.confidence}%`
                      }}
                    </span>
                  </span>
                </button>
                <button
                  class="btn-outline btn-sm open-pdf-btn"
                  :title="`Seite ${page.page} im Reader öffnen`"
                  @click="openInReader(group.filePath, page.page)"
                >
                  <i class="icon-acrobat" aria-hidden="true"></i>öffnen
                </button>
              </div>
            </li>
          </ul>
        </li>
      </ul>
    </div>

    <p class="notice notice-empty" v-if="noResultsFound">
      <span aria-hidden="true">🔎</span>
      <span>
        Keine Resultate gefunden.
        <template v-if="scanned === 0">
          Im Ordner liegen keine PDF-Dateien.
        </template>
        <template v-else>({{ scanned }} Dateien durchsucht)</template>
      </span>
    </p>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import { searchKeyword, addToSearchHistory } from "../composables/useSearch";
import { showPdfFromUrl } from "../composables/usePdfViewer";
import { errorMessage, throwError, clearError } from "../composables/useError";
import {
  SearchPDFs,
  CancelSearch,
  GetPDFsFolder,
  OpenPDF,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import UploadComponent from "./UploadComponent.vue";

const progress = ref(0);
const isSearching = ref(false);
const noResultsFound = ref(false);
const scanned = ref(0);
const failures = ref([]);
const groupedResults = ref([]);
const expandedFiles = ref({});

/** Groups the flat result list by file, best match first. */
const groupByFile = (results) => {
  const grouped = {};

  results.forEach((result) => {
    if (!grouped[result.file]) {
      grouped[result.file] = {
        fileName: result.fileName,
        filePath: result.file,
        maxConfidence: result.confidence,
        pages: [],
      };
    }

    if (result.confidence > grouped[result.file].maxConfidence) {
      grouped[result.file].maxConfidence = result.confidence;
    }

    grouped[result.file].pages.push({
      page: result.page,
      foundWord: result.foundWord,
      confidence: result.confidence,
    });
  });

  const groups = Object.values(grouped).sort(
    (a, b) => b.maxConfidence - a.maxConfidence
  );
  groups.forEach((group) => {
    group.pages.sort((a, b) => b.confidence - a.confidence);
  });
  return groups;
};

const startSearch = async () => {
  clearError();

  if (!searchKeyword.value.trim()) {
    throwError("Bitte ein Schlüsselwort eingeben!");
    return;
  }

  groupedResults.value = [];
  expandedFiles.value = {};
  failures.value = [];
  noResultsFound.value = false;
  progress.value = 0;
  isSearching.value = true;

  addToSearchHistory(searchKeyword.value);

  try {
    const folder = await GetPDFsFolder();
    const outcome = await SearchPDFs(folder, searchKeyword.value);

    // A cancelled run returns whatever it had managed so far — dropping it is
    // the only honest option, since it is not a complete answer.
    if (outcome.cancelled) return;

    scanned.value = outcome.scanned;
    failures.value = outcome.failures || [];
    groupedResults.value = groupByFile(outcome.results || []);
    noResultsFound.value = groupedResults.value.length === 0;
  } catch (error) {
    throwError("Suche fehlgeschlagen", error);
  } finally {
    isSearching.value = false;
    progress.value = 0;
  }
};

const cancelSearch = () => {
  CancelSearch();
};

const openInReader = async (pdfFilePath, pageNumber) => {
  try {
    await OpenPDF(pdfFilePath, pageNumber);
  } catch (error) {
    throwError("Fehler beim Öffnen des PDFs", error);
  }
};

const onProgress = (data) => {
  if (isSearching.value && typeof data.progress === "number") {
    progress.value = data.progress;
  }
};

onMounted(() => EventsOn("search-progress", onProgress));
onUnmounted(() => EventsOff("search-progress"));
</script>
