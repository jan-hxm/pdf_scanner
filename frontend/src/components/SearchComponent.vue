<template>
  <div class="search-sidebar">
    <UploadComponent></UploadComponent>
    <div class="hr"></div>

    <div class="search-section">
      <h4><label for="search-keyword">Schlüsselwort</label></h4>
      <form @submit.prevent="startSearch()" class="search-form">
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
      <p class="progress-text">
        <span>Durchsucht</span>
        <span class="progress-count">
          {{ filesDone }}<template v-if="filesTotal">
            / {{ filesTotal }}</template
          >
          Dateien
        </span>
      </p>
      <progress max="100" :value="progress" class="progress-bar"></progress>
      <button class="btn-outline btn-sm cancel-btn" @click="cancelSearch">
        Suche abbrechen
      </button>
    </div>

    <p class="notice notice-empty" v-if="wasCancelled">
      <span aria-hidden="true">⏹️</span>
      <span>
        Suche abgebrochen — ein unvollständiges Ergebnis wird nicht angezeigt.
      </span>
    </p>

    <p
      class="notice notice-warning"
      v-if="failures.length > 0"
      :title="allFailedNames"
    >
      <span aria-hidden="true">⚠️</span>
      <span>
        {{ failures.length }}
        {{ failures.length === 1 ? "Datei konnte" : "Dateien konnten" }} nicht
        gelesen werden: {{ failedNames }}
      </span>
    </p>

    <OcrPanel :files="textless" @completed="onRecognized" />

    <p class="notice notice-empty" v-if="ocrSummary">
      <span aria-hidden="true">🔤</span>
      <span>{{ ocrSummary }}</span>
    </p>

    <div class="result-heading-container" v-if="sortedGroups.length > 0">
      <h3>
        {{ totalHits }} {{ totalHits === 1 ? "Treffer" : "Treffer" }} in
        {{ sortedGroups.length }}
        {{ sortedGroups.length === 1 ? "Datei" : "Dateien" }}
      </h3>
      <p>
        für <span class="result-term">{{ resultKeyword }}</span> ·
        {{ scanned }} {{ scanned === 1 ? "Datei" : "Dateien" }} durchsucht
      </p>

      <div class="result-toolbar">
        <label class="sort-control">
          <span class="sort-label">Sortierung</span>
          <select v-model="sortMode" class="sort-select">
            <option value="relevance">Relevanz</option>
            <option value="hits">Treffer</option>
            <option value="name">Name (A–Z)</option>
          </select>
        </label>
        <button
          type="button"
          class="btn-outline btn-sm expand-all-btn"
          @click="toggleAll"
        >
          {{ allExpanded ? "Alle einklappen" : "Alle ausklappen" }}
        </button>
      </div>
    </div>

    <div class="results-section" v-if="sortedGroups.length > 0">
      <ul class="results-list">
        <li
          v-for="group in sortedGroups"
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
            <span class="hit-count" :aria-label="hitCountLabel(group)">
              {{ group.hits }}
            </span>
            <span class="file-name" :title="group.filePath">
              {{ group.fileName }}
            </span>
            <span class="page-count">{{ pageCountLabel(group) }}</span>
            <span class="disclosure" aria-hidden="true">▾</span>
          </button>

          <ul v-if="expandedFiles[group.filePath]" class="page-list">
            <li
              v-for="page in group.pages"
              :key="page.key"
              class="page-item"
              :class="{ 'is-active': page.key === activeHitKey }"
            >
              <div class="page-info-container">
                <button
                  type="button"
                  class="page-info"
                  :aria-current="page.key === activeHitKey ? 'true' : undefined"
                  @click="openHit(group, page)"
                >
                  <span class="found-word-row">
                    <span class="found-word">{{ page.foundWord }}</span>
                    <span
                      class="occurrence-count"
                      v-if="page.occurrences > 1"
                      :title="`${page.occurrences} Treffer auf dieser Seite`"
                      >{{ page.occurrences }}×</span
                    >
                  </span>
                  <span class="page-meta">
                    Seite {{ page.page }} ·
                    <span :title="`Übereinstimmung: ${page.confidence}%`">
                      {{
                        page.confidence >= 100 ? "exakt" : `~${page.confidence}%`
                      }}
                    </span>
                    <span
                      class="ocr-badge"
                      v-if="page.fromOcr"
                      title="In erkanntem Text gefunden. Diese Seite ist ein Scan und lässt sich nicht hervorheben."
                      >OCR</span
                    >
                  </span>
                  <span class="snippet" v-if="page.snippet.length > 0">
                    <template v-for="(part, i) in page.snippet">
                      <mark v-if="part.match" :key="`m-${i}`">{{
                        part.text
                      }}</mark>
                      <span v-else :key="`p-${i}`">{{ part.text }}</span>
                    </template>
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
import { ref, computed, onMounted, onUnmounted } from "vue";
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
import OcrPanel from "./OcrPanel.vue";
import { ocrSummary, clearOcrSummary } from "../composables/useOcr";

const progress = ref(0);
const filesDone = ref(0);
const filesTotal = ref(0);
const isSearching = ref(false);
const noResultsFound = ref(false);
const wasCancelled = ref(false);
const scanned = ref(0);
const resultKeyword = ref("");
const failures = ref([]);
const textless = ref([]);
const groups = ref([]);
const expandedFiles = ref({});
const activeHitKey = ref("");
const sortMode = ref("relevance");

// Only the newest run may touch the shared state. A superseded run resolves
// *after* its successor has started, so without this guard its finally block
// would switch the progress bar off in the middle of the newer search.
let runToken = 0;

const MAX_LISTED_FAILURES = 3;
const SNIPPET_MAX = 110;

const joinNames = (list) => list.map((f) => f.fileName).join(", ");

// Long file lists are truncated in the notice itself and given in full in its
// title attribute, so the line cannot grow past the pane.
const summariseNames = (list) => {
  const names = list.map((f) => f.fileName);
  if (names.length <= MAX_LISTED_FAILURES) return names.join(", ");
  const rest = names.length - MAX_LISTED_FAILURES;
  return `${names.slice(0, MAX_LISTED_FAILURES).join(", ")} und ${rest} weitere`;
};

const allFailedNames = computed(() => joinNames(failures.value));
const failedNames = computed(() => summariseNames(failures.value));

const hitCountLabel = (group) =>
  `${group.hits} Treffer auf ${group.pageCount} ${
    group.pageCount === 1 ? "Seite" : "Seiten"
  }`;

const pageCountLabel = (group) =>
  `${group.pageCount} ${group.pageCount === 1 ? "Seite" : "Seiten"}`;

/**
 * Splits the matched line into plain and highlighted parts, windowed around
 * the first match so one long line cannot stretch the sidebar. Returned as
 * parts rather than markup, so text lifted out of a PDF is never rendered as
 * HTML.
 */
const buildSnippet = (context, word) => {
  const text = (context || "").replace(/\s+/g, " ").trim();
  if (!text) return [];

  const lower = text.toLowerCase();
  const needle = (word || "").toLowerCase();
  const first = needle ? lower.indexOf(needle) : -1;

  let start = 0;
  let end = text.length;
  if (text.length > SNIPPET_MAX) {
    const centre = first >= 0 ? first + needle.length / 2 : 0;
    end = Math.min(text.length, Math.round(centre + SNIPPET_MAX / 2));
    start = Math.max(0, end - SNIPPET_MAX);
  }

  const parts = [];
  if (start > 0) parts.push({ text: "…", match: false });

  let cursor = start;
  if (needle) {
    let i = lower.indexOf(needle, start);
    while (i !== -1 && i < end) {
      if (i > cursor) parts.push({ text: text.slice(cursor, i), match: false });
      parts.push({
        text: text.slice(i, Math.min(i + needle.length, end)),
        match: true,
      });
      cursor = Math.max(cursor, i + needle.length);
      i = lower.indexOf(needle, cursor);
    }
  }
  if (cursor < end) parts.push({ text: text.slice(cursor, end), match: false });
  if (end < text.length) parts.push({ text: "…", match: false });
  return parts;
};

/**
 * Groups the flat result list by file. `hits` counts every occurrence in the
 * file, not the number of pages that hold one — a term found eight times on a
 * single page is eight hits, and that is what the file row shows. `pageCount`
 * carries the other number, so neither has to stand in for the other.
 */
const groupByFile = (results) => {
  const grouped = new Map();

  results.forEach((result) => {
    let group = grouped.get(result.file);
    if (!group) {
      group = {
        fileName: result.fileName,
        filePath: result.file,
        maxConfidence: result.confidence,
        hits: 0,
        pageCount: 0,
        pages: [],
      };
      grouped.set(result.file, group);
    }

    const occurrences = result.occurrences > 0 ? result.occurrences : 1;
    group.maxConfidence = Math.max(group.maxConfidence, result.confidence);
    group.hits += occurrences;
    group.pages.push({
      key: `${result.file}|${result.page}|${result.foundWord}`,
      page: result.page,
      foundWord: result.foundWord,
      confidence: result.confidence,
      occurrences,
      // A hit found in recognised text is worth marking: OCR misreads
      // characters, so the snippet may differ slightly from the page, and the
      // viewer cannot highlight it — a scan has no text layer to draw on.
      fromOcr: result.match === "ocr",
      snippet: buildSnippet(result.context, result.foundWord),
    });
  });

  const list = [...grouped.values()];
  list.forEach((group) => {
    // Best match first, then in reading order: equally confident hits are most
    // useful walked front to back through the document.
    group.pages.sort((a, b) => b.confidence - a.confidence || a.page - b.page);
    group.pageCount = new Set(group.pages.map((p) => p.page)).size;
  });
  return list;
};

const byName = (a, b) =>
  a.fileName.localeCompare(b.fileName, "de", { numeric: true });

const sorters = {
  // Confidence decides, then how much the file actually has to offer — one
  // exact hit should not outrank twenty of them purely by arrival order.
  relevance: (a, b) =>
    b.maxConfidence - a.maxConfidence || b.hits - a.hits || byName(a, b),
  hits: (a, b) =>
    b.hits - a.hits || b.maxConfidence - a.maxConfidence || byName(a, b),
  name: byName,
};

const sortedGroups = computed(() =>
  [...groups.value].sort(sorters[sortMode.value] || sorters.relevance)
);

const totalHits = computed(() =>
  groups.value.reduce((sum, group) => sum + group.hits, 0)
);

const allExpanded = computed(
  () =>
    sortedGroups.value.length > 0 &&
    sortedGroups.value.every((group) => expandedFiles.value[group.filePath])
);

const toggleAll = () => {
  const open = !allExpanded.value;
  const next = {};
  if (open) sortedGroups.value.forEach((group) => (next[group.filePath] = true));
  expandedFiles.value = next;
};

const openHit = (group, page) => {
  activeHitKey.value = page.key;
  showPdfFromUrl(group.filePath, page.page, page.foundWord);
};

/**
 * Runs the search.
 *
 * keepOcrSummary is set by the re-run that follows text recognition: that run
 * is the whole point of the summary, so clearing it there would wipe the one
 * line explaining why a scanned file suddenly has hits.
 */
const startSearch = async ({ keepOcrSummary = false } = {}) => {
  clearError();
  if (!keepOcrSummary) clearOcrSummary();

  if (!searchKeyword.value.trim()) {
    throwError("Bitte ein Schlüsselwort eingeben!");
    return;
  }

  const token = ++runToken;
  const keyword = searchKeyword.value;

  groups.value = [];
  expandedFiles.value = {};
  activeHitKey.value = "";
  failures.value = [];
  textless.value = [];
  noResultsFound.value = false;
  wasCancelled.value = false;
  progress.value = 0;
  filesDone.value = 0;
  filesTotal.value = 0;
  isSearching.value = true;

  if (!keepOcrSummary) addToSearchHistory(keyword);

  try {
    const folder = await GetPDFsFolder();
    const outcome = await SearchPDFs(folder, keyword);
    if (token !== runToken) return;

    // A cancelled run returns whatever it had managed so far — dropping it is
    // the only honest option, since it is not a complete answer. Say so
    // instead of clearing the pane and leaving the user guessing.
    if (outcome.cancelled) {
      wasCancelled.value = true;
      return;
    }

    scanned.value = outcome.scanned;
    resultKeyword.value = keyword;
    failures.value = outcome.failures || [];
    textless.value = outcome.textless || [];
    groups.value = groupByFile(outcome.results || []);
    noResultsFound.value = groups.value.length === 0;

    // Open the best file straight away: a list of collapsed rows makes the
    // user click twice to see the thing they just searched for.
    const top = sortedGroups.value[0];
    if (top) expandedFiles.value = { [top.filePath]: true };
  } catch (error) {
    if (token !== runToken) return;
    throwError("Suche fehlgeschlagen", error);
  } finally {
    if (token === runToken) {
      isSearching.value = false;
      progress.value = 0;
    }
  }
};

const cancelSearch = () => {
  CancelSearch();
};

/**
 * Re-runs the search once text recognition has finished. The recognised text
 * is cached by then, so this run reads it back rather than recognising again —
 * the files that were listed as unsearchable a moment ago now return hits.
 */
const onRecognized = () => {
  if (!searchKeyword.value.trim()) return;
  startSearch({ keepOcrSummary: true });
};

const openInReader = async (pdfFilePath, pageNumber) => {
  try {
    await OpenPDF(pdfFilePath, pageNumber);
  } catch (error) {
    throwError("Fehler beim Öffnen des PDFs", error);
  }
};

const onProgress = (data) => {
  if (!isSearching.value) return;
  if (typeof data.progress === "number") progress.value = data.progress;
  if (typeof data.completed === "number") filesDone.value = data.completed;
  if (typeof data.total === "number") filesTotal.value = data.total;
};

onMounted(() => EventsOn("search-progress", onProgress));
onUnmounted(() => EventsOff("search-progress"));
</script>
