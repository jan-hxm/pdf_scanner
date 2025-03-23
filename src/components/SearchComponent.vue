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
        <button type="submit" class="btn-primary">🔍 Suchen</button>
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
              <div
                @click="
                  showPdfFromUrl(group.filePath, page.page, page.positions)
                "
              >
                <span class="found-word"
                  >📌 Gefunden: <b>{{ page.foundWord }}</b></span
                >
                <span>Seite: {{ page.page }}</span>
              </div>
              <button
                class="btn-outline"
                @click="loadPdf(group.filePath, page.page)"
              >
                <i class="icon-acrobat"></i>öffnen
              </button>
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
import { useIpcRenderer } from "../composables/useIpcRenderer";
import { showPdfFromUrl } from "../composables/usePdfViewer";
import { errorMessage, throwError } from "../composables/useError";
import UploadComponent from "./UploadComponent.vue";

const ipcRenderer = useIpcRenderer();

const progress = ref(0);
const noResultsFound = ref(false);
const groupedResults = ref({});
const expandedFiles = ref({});
const activeSearchTerm = ref("");

const startSearch = () => {
  errorMessage.value = false;
  if (searchKeyword.value && searchKeyword.value !== activeSearchTerm.value) {
    activeSearchTerm.value = searchKeyword.value;
    groupedResults.value = {};
    expandedFiles.value = {};
    progress.value = 1;
    noResultsFound.value = false;

    addToSearchHistory(searchKeyword.value);

    console.log(
      "📌 Sende Event 'start-pdf-search' mit Keyword:",
      searchKeyword.value
    );
    ipcRenderer.send("start-pdf-search", searchKeyword.value);

    ipcRenderer.on("pdf-search-progress", (event, data) => {
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
              maxConfidence: result.confidence, // Höchste Confidence pro Datei
              pages: [],
            };
          }

          // Aktuelle höchste Confidence pro Datei speichern
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

        // Konvertiere Objekt zu sortiertem Array basierend auf maxConfidence
        groupedResults.value = Object.values(grouped).sort(
          (a, b) => b.maxConfidence - a.maxConfidence
        );

        // Innerhalb jedes Dokuments die Seiten nach Confidence sortieren
        groupedResults.value.forEach((group) => {
          group.pages.sort((a, b) => b.confidence - a.confidence);
        });
      }
    });
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
    const response = await ipcRenderer.invoke(
      "pdf:openPdf",
      pdfFilePath,
      pageNumber
    );
    if (response.error) {
      errorMessage.value = response.error;
    } else {
      console.log(`PDF wurde auf Seite ${response.page} geöffnet.`);
    }
  } catch (error) {
    throwError(`Fehler beim Öffnen des PDFs: ${error}`);
  }
};

onMounted(() => {
  ipcRenderer.on("file-move-error", (_, message) => {
    throwError(`Fehler: ${message}`);
  });
});
</script>
