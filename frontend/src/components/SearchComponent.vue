<template>
  <div class="search-sidebar">
    <UploadComponent></UploadComponent>
    <div class="hr"></div>
    <h4>Schlüsselwort eingeben:</h4>

    <div class="search-section">
      <form @submit.prevent="startSearch" class="search-form">
        <input
          v-model="searchKeyword"
          class="input-field"
          placeholder="Begriff hier eingeben 🔎"
        />
        <button type="submit" class="btn-primary search-btn">🔍 Suchen</button>
      </form>
      <p class="error" v-if="errorMessage">{{ errorMessage }}</p>
    </div>

    <div class="progress-section" v-if="progress > 0 && progress < 100">
      <p class="progress-text">Fortschritt: {{ progress }}%</p>
      <progress max="100" :value="progress" class="progress-bar"></progress>
    </div>

    <div class="result-heading-container" v-if="groupedResults.length > 0">
      <h3>{{ groupedResults.length }} Resultate:</h3>
      <p>Treffer anklicken um das Dokument zu untersuchen.</p>
    </div>
    <div class="results-section" v-if="groupedResults.length > 0">
      <ul class="results-list">
        <li
          v-for="group in groupedResults"
          :key="group.filePath"
          class="result-item"
        >
          <div>
            <small> ({{ group.pages.length }}) </small
            ><span
              class="file-name"
              @click="
                expandedFiles[group.filePath] = !expandedFiles[group.filePath]
              "
            >
              📑{{ group.fileName }}
              {{ expandedFiles[group.filePath] ? "🔺" : "🔻" }}
            </span>
          </div>
          <ul v-if="expandedFiles[group.filePath]" class="page-list">
            <li
              v-for="page in group.pages"
              :key="`${group.filePath}-${page.page}`"
              class="page-item"
            >
              <div class="flex-container page-info-container">
                <div
                  class="page-info"
                  @click="
                    showPdfFromUrl(group.filePath, page.page, page.positions)
                  "
                >
                  <span class="found-word"
                    >📌 Gefunden: <b>{{ page.foundWord }}</b></span
                  >
                  <small>Seite: {{ page.page }}</small>
                </div>
                <button
                  class="btn-outline open-pdf-btn"
                  @click="loadPdf(group.filePath, page.page)"
                >
                  <i class="icon-acrobat"></i>öffnen
                </button>
              </div>
            </li>
          </ul>
        </li>
      </ul>
    </div>
    <div v-if="noResultsFound">
      <p class="error">Keine Resultate gefunden!</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from "vue";
import { searchKeyword, addToSearchHistory } from "../composables/useSearch";
import { showPdfFromUrl } from "../composables/usePdfViewer";
import { errorMessage, throwError } from "../composables/useError";
import { SearchPDFs, GetPDFsFolder, OpenPDF } from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import UploadComponent from "./UploadComponent.vue";

const progress = ref(0);
const noResultsFound = ref(false);
const groupedResults = ref({});
const expandedFiles = ref({});
const activeSearchTerm = ref("");

const startSearch = async () => {
  errorMessage.value = false;
  if (searchKeyword.value && searchKeyword.value !== activeSearchTerm.value) {
    activeSearchTerm.value = searchKeyword.value;
    groupedResults.value = {};
    expandedFiles.value = {};
    progress.value = 1;
    noResultsFound.value = false;

    addToSearchHistory(searchKeyword.value);

    const folder = await GetPDFsFolder();

    // Set up listener before triggering search to avoid missing early events
    EventsOff("search-progress");
    EventsOn("search-progress", (data) => {
      if (data.progress) {
        progress.value = data.progress;
      }

      if (data.results) {
        console.log("Results:", data.results);
        if (data.results.length == 0) noResultsFound.value = true;
        const resultsArray = data.results;
        const grouped = {};

        resultsArray.forEach((result) => {
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
            positions: result.positions,
          });
        });

        groupedResults.value = Object.values(grouped).sort(
          (a, b) => b.maxConfidence - a.maxConfidence
        );

        groupedResults.value.forEach((group) => {
          group.pages.sort((a, b) => b.confidence - a.confidence);
        });
      }
    });

    console.log(
      "📌 Starte Suche mit Keyword:",
      searchKeyword.value,
      "in Ordner:",
      folder
    );
    SearchPDFs(folder, searchKeyword.value);
  } else {
    let err = "Bitte ein Schlüsselwort eingeben!";
    if (
      activeSearchTerm.value !== "" &&
      searchKeyword.value == activeSearchTerm.value
    )
      err = "";

    throwError(err);
  }
};

const loadPdf = async (pdfFilePath, pageNumber) => {
  try {
    await OpenPDF(pdfFilePath, pageNumber);
    console.log(`PDF wurde auf Seite ${pageNumber} geöffnet.`);
  } catch (error) {
    throwError(`Fehler beim Öffnen des PDFs: ${error}`);
  }
};
</script>
