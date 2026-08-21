import { ref } from "vue";
import * as pdfjsLib from "pdfjs-dist";
import pdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorker;
import { clearError, throwError } from "../composables/useError";
import { LoadPDF } from "../../wailsjs/go/main/App";

export const canvasRef = ref(null);
export const highlights = ref([]);
export const isLoading = ref(false);
export const renderCanvas = ref(false);
export const currentPage = ref(1);
export const totalPages = ref(1);
export const scale = ref(1.2);
export const viewerMessage = ref("");

// The term the search found, highlighted on whichever page is on screen.
const activeTerm = ref("");

let pdfDocument = null;

/**
 * Escapes a term for use in a regex and lets any whitespace in it match any
 * run of whitespace, so "E. coli" still matches text extracted as "E.  coli".
 */
const termToRegExp = (term) => {
  const escaped = term
    .trim()
    .replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
    .replace(/\s+/g, "\\s+");
  return new RegExp(escaped, "gi");
};

/**
 * Bounding box, in viewport (CSS pixel) space, of characters [from, to) of a
 * PDF.js text item.
 *
 * `item.transform` maps the text run into PDF user space; combining it with
 * `viewport.transform` folds in scale, page rotation and the CropBox origin,
 * so the result is where the glyphs actually sit on the rendered canvas.
 */
const boxForItemRange = (item, from, to, viewport) => {
  const tx = pdfjsLib.Util.transform(viewport.transform, item.transform);
  const angle = Math.atan2(tx[1], tx[0]);
  const fontHeight = Math.hypot(tx[2], tx[3]);
  const totalWidth = item.width * viewport.scale;

  const chars = item.str.length || 1;
  const along0 = (totalWidth * from) / chars;
  const along1 = (totalWidth * to) / chars;

  // Unit vector along the baseline, and the one pointing "up" the glyphs.
  // Viewport y grows downwards, hence the sign on `up`.
  const ux = Math.cos(angle);
  const uy = Math.sin(angle);
  const vx = Math.sin(angle);
  const vy = -Math.cos(angle);

  // A little below the baseline for descenders, a little above for accents.
  const below = -0.22 * fontHeight;
  const above = 1.0 * fontHeight;

  const xs = [];
  const ys = [];
  for (const along of [along0, along1]) {
    for (const up of [below, above]) {
      xs.push(tx[4] + ux * along + vx * up);
      ys.push(tx[5] + uy * along + vy * up);
    }
  }

  const x = Math.min(...xs);
  const y = Math.min(...ys);
  return {
    x,
    y,
    width: Math.max(...xs) - x,
    height: Math.max(...ys) - y,
  };
};

/**
 * Finds every occurrence of `term` in the page's text layer and returns one
 * box per text item it spans.
 */
const findHighlights = async (pdfPage, viewport, term) => {
  if (!term) return [];

  const { items } = await pdfPage.getTextContent();

  // Flatten the items into one string, remembering which slice came from which
  // item so a match can be mapped back to glyph positions.
  let haystack = "";
  const spans = [];
  for (const item of items) {
    if (typeof item.str !== "string") continue; // marked-content markers
    if (item.str.length > 0) {
      spans.push({ item, start: haystack.length, end: haystack.length + item.str.length });
      haystack += item.str;
    }
    if (item.hasEOL) haystack += "\n";
  }

  const boxes = [];
  const pattern = termToRegExp(term);
  let match;
  while ((match = pattern.exec(haystack)) !== null) {
    if (match[0].length === 0) break; // guard against a zero-width pattern
    const matchStart = match.index;
    const matchEnd = matchStart + match[0].length;

    for (const span of spans) {
      if (span.end <= matchStart || span.start >= matchEnd) continue;
      const from = Math.max(matchStart, span.start) - span.start;
      const to = Math.min(matchEnd, span.end) - span.start;
      boxes.push(boxForItemRange(span.item, from, to, viewport));
    }
  }
  return boxes;
};

const renderPage = async (page) => {
  if (!pdfDocument) return;

  try {
    const pdfPage = await pdfDocument.getPage(page);
    const viewport = pdfPage.getViewport({ scale: scale.value });

    const canvas = canvasRef.value;
    const context = canvas.getContext("2d");

    canvas.width = viewport.width;
    canvas.height = viewport.height;

    await pdfPage.render({ canvasContext: context, viewport }).promise;

    highlights.value = await findHighlights(pdfPage, viewport, activeTerm.value);
    viewerMessage.value =
      activeTerm.value && highlights.value.length === 0
        ? `„${activeTerm.value}" ist auf dieser Seite nicht als Text markierbar (evtl. ein Scan).`
        : "";

    currentPage.value = page;
  } catch (error) {
    throwError("Fehler beim Rendern der Seite", error);
  }
};

export const loadPdf = async (uint8Array, page) => {
  try {
    isLoading.value = true;
    pdfDocument = await pdfjsLib.getDocument({ data: uint8Array }).promise;
    totalPages.value = pdfDocument.numPages;
    await renderPage(Math.min(Math.max(page, 1), totalPages.value));
  } catch (error) {
    throwError("Fehler beim Laden des PDFs", error);
  } finally {
    isLoading.value = false;
  }
};

export const nextPage = () => {
  renderPage(currentPage.value < totalPages.value ? currentPage.value + 1 : 1);
};

export const prevPage = () => {
  renderPage(currentPage.value > 1 ? currentPage.value - 1 : totalPages.value);
};

export const scaleUp = () => {
  if (scale.value < 2) {
    scale.value = +(scale.value + 0.1).toFixed(1);
    renderPage(currentPage.value);
  }
};

export const scaleDown = () => {
  if (scale.value > 0.1) {
    scale.value = +(scale.value - 0.1).toFixed(1);
    renderPage(currentPage.value);
  }
};

export const showPdfFromUrl = async (filePath, page, term) => {
  activeTerm.value = term || "";
  highlights.value = [];
  viewerMessage.value = "";
  renderCanvas.value = false;
  clearError();

  try {
    // LoadPDF returns []byte, which Wails serialises as base64.
    const base64 = await LoadPDF(filePath);
    const uint8Array = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    renderCanvas.value = true;
    await loadPdf(uint8Array, page);
  } catch (error) {
    renderCanvas.value = false;
    throwError("❌ Fehler beim Laden der PDF-Datei", error);
  }
};
