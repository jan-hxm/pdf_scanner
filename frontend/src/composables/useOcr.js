import { ref } from "vue";
import {
  OCRStatus,
  RecognizeText,
  CancelOCR,
  OCRCacheStats,
  ClearOCRCache,
} from "../../wailsjs/go/main/App";

/**
 * Text recognition for scanned PDFs.
 *
 * The state lives at module level rather than inside the component because the
 * run outlives the panel that started it: once recognition finishes, the search
 * is re-run, the file stops being textless, and the panel unmounts. A summary
 * held in the component would disappear with it, leaving the user with new
 * results and no explanation of where they came from.
 */

const emptyStatus = {
  available: false,
  path: "",
  version: "",
  languages: [],
  reason: "",
};

const emptyProgress = {
  progress: 0,
  fileIndex: 0,
  fileTotal: 0,
  fileName: "",
  page: 0,
  pageTotal: 0,
};

export const ocrStatus = ref({ ...emptyStatus });
export const isRecognizing = ref(false);
export const ocrProgress = ref({ ...emptyProgress });
export const ocrSummary = ref("");
export const cacheStats = ref({ entries: 0, bytes: 0 });

/** German names for the language codes Tesseract is likely to have installed. */
const LANGUAGE_NAMES = {
  deu: "Deutsch",
  eng: "Englisch",
  fra: "Französisch",
  ita: "Italienisch",
  spa: "Spanisch",
  nld: "Niederländisch",
  por: "Portugiesisch",
  pol: "Polnisch",
  ces: "Tschechisch",
  tur: "Türkisch",
  rus: "Russisch",
  lat: "Latein",
};

export const languageName = (code) => LANGUAGE_NAMES[code] || code;

/**
 * Builds the picker's options from the installed languages.
 *
 * Tesseract takes several languages at once and lets them compete per word, so
 * a document that mixes German and English is offered as one combined choice
 * rather than forcing a wrong answer for half its pages.
 */
export const languageOptions = (codes) => {
  const list = (codes || []).map((code) => ({
    value: code,
    label: `${languageName(code)} (${code})`,
  }));

  if ((codes || []).includes("deu") && (codes || []).includes("eng")) {
    list.unshift({
      value: "deu+eng",
      label: "Deutsch + Englisch (gemischt)",
    });
  }
  return list;
};

export async function loadOcrStatus() {
  try {
    const status = await OCRStatus();
    ocrStatus.value = { ...emptyStatus, ...status };
  } catch {
    // A status probe that fails is indistinguishable from a missing engine as
    // far as the user is concerned, and both render the same way.
    ocrStatus.value = {
      ...emptyStatus,
      reason: "Der OCR-Status konnte nicht ermittelt werden.",
    };
  }
  return ocrStatus.value;
}

/** Describes a finished run in one line, counting only what actually changed. */
const describeReport = (report) => {
  const files = report?.files || [];
  const failed = files.filter((f) => f.error);
  const done = files.filter((f) => !f.error && !f.cached);
  const pages = done.reduce((sum, f) => sum + (f.pages || 0), 0);

  if (report?.cancelled) {
    return "Texterkennung abgebrochen — angefangene Dateien wurden nicht gespeichert.";
  }

  const parts = [];
  if (done.length > 0) {
    parts.push(
      `${pages} ${pages === 1 ? "Seite" : "Seiten"} in ${done.length} ${
        done.length === 1 ? "Datei" : "Dateien"
      } erkannt`
    );
  }
  if (failed.length > 0) {
    parts.push(
      `${failed.length} ${
        failed.length === 1 ? "Datei" : "Dateien"
      } fehlgeschlagen: ${failed.map((f) => f.error).join("; ")}`
    );
  }
  if (parts.length === 0) return "Es gab nichts zu erkennen.";
  return `Texterkennung abgeschlossen — ${parts.join(", ")}.`;
};

/**
 * Recognises text in the given files and returns the report.
 *
 * Deliberately not awaited by the search: it runs after results are already on
 * screen, because seconds per page over a 31-page scan is minutes the user
 * would otherwise spend staring at a progress bar with nothing to read.
 */
export async function recognizeFiles(paths, languages) {
  isRecognizing.value = true;
  ocrSummary.value = "";
  ocrProgress.value = { ...emptyProgress, fileTotal: paths.length };

  try {
    const report = await RecognizeText(paths, languages);
    ocrSummary.value = describeReport(report);
    return report;
  } finally {
    isRecognizing.value = false;
    ocrProgress.value = { ...emptyProgress };
    // The run just grew the cache, and the settings panel may be sitting open
    // next to it showing a count that is now wrong.
    loadCacheStats();
  }
}

/** Reads how much recognised text is on disk. Never throws: an unreadable cache
 *  is an empty one as far as anything the user can do about it goes. */
export async function loadCacheStats() {
  try {
    const stats = await OCRCacheStats();
    cacheStats.value = {
      entries: stats?.entries || 0,
      bytes: stats?.bytes || 0,
    };
  } catch {
    cacheStats.value = { entries: 0, bytes: 0 };
  }
  return cacheStats.value;
}

/**
 * Discards every cached recognition and returns how many documents went.
 *
 * Errors are the caller's to show — this one is worth a toast, because unlike a
 * status probe the user asked for it and is entitled to know it did not happen.
 */
export async function clearCache() {
  const removed = await ClearOCRCache();
  await loadCacheStats();
  return removed;
}

/** Sizes for a sidebar, where "26,4 KB" is the whole point and "27043" is not. */
export const formatBytes = (bytes) => {
  if (!bytes) return "0 KB";
  if (bytes < 1024 * 1024) {
    return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`;
};

export function cancelRecognition() {
  CancelOCR();
}

export function applyOcrProgress(data) {
  ocrProgress.value = {
    progress: typeof data.progress === "number" ? data.progress : 0,
    fileIndex: data.fileIndex || 0,
    fileTotal: data.fileTotal || 0,
    fileName: data.fileName || "",
    page: data.page || 0,
    pageTotal: data.pageTotal || 0,
  };
}

export function clearOcrSummary() {
  ocrSummary.value = "";
}
