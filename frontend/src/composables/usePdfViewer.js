import { ref } from "vue";
import * as pdfjsLib from "pdfjs-dist";
import pdfWorker from "pdfjs-dist/build/pdf.worker.min.mjs?url";
pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorker;
import { errorMessage, throwError } from "../composables/useError";
import { LoadPDF } from "../../wailsjs/go/main/App";

const initialPageHighlights = ref({});
export const canvasRef = ref(null);
export const highlights = ref([]);
export const isLoading = ref(false);
export const renderCanvas = ref(false);
export const currentPage = ref(1);
export const totalPages = ref(1);
export const scale = ref(1.2);

let pdfDocument = null; // Store the loaded PDF document

export const loadPdf = async (uint8Array, page, positions) => {
  try {
    isLoading.value = true;
    errorMessage.value = "";

    console.log("Lade PDF als Uint8Array...");
    pdfDocument = await pdfjsLib.getDocument({ data: uint8Array }).promise;
    totalPages.value = pdfDocument.numPages;

    await renderPage(page, positions);
  } catch (error) {
    throwError("Fehler beim Laden des PDFs", error);
  } finally {
    isLoading.value = false;
  }
};

const renderPage = async (page, positions) => {
  if (!pdfDocument) return;

  if (page == initialPageHighlights.value.page) {
    positions = initialPageHighlights.value.positions;
  }

  try {
    const pdfPage = await pdfDocument.getPage(page);
    const viewport = pdfPage.getViewport({ scale: scale.value });

    const canvas = canvasRef.value;
    const context = canvas.getContext("2d");

    canvas.width = viewport.width;
    canvas.height = viewport.height;

    const renderContext = {
      canvasContext: context,
      viewport: viewport,
    };

    await pdfPage.render(renderContext).promise;

    highlights.value = [];
    positions.forEach((pos) => {
      const xPadding = 4;
      const yPadding = 2;
      const x = pos.x0 - xPadding / 2;
      const y = pos.y0;

      const width = pos.x1 + xPadding - x;
      const height = pos.y1 + yPadding - y;

      highlightText(
        x * scale.value,
        y * scale.value,
        width * scale.value,
        height * scale.value
      );
    });

    currentPage.value = page;
  } catch (error) {
    throwError("Fehler beim Rendern der Seite", error);
  }
};

export const nextPage = () => {
  if (currentPage.value < totalPages.value) {
    renderPage(currentPage.value + 1, []);
  } else {
    currentPage.value = 0;
    renderPage(currentPage.value + 1, []);
  }
};

export const prevPage = () => {
  if (currentPage.value > 1) {
    renderPage(currentPage.value - 1, []);
  } else {
    currentPage.value = totalPages.value;
    renderPage(currentPage.value, []);
  }
};

export const scaleUp = () => {
  if (scale.value < 2) {
    scale.value = +(scale.value + 0.1).toFixed(1);
    renderPage(currentPage.value, []);
  }
};

export const scaleDown = () => {
  if (scale.value > 0.1) {
    scale.value = +(scale.value - 0.1).toFixed(1);
    renderPage(currentPage.value, []);
  }
};

function highlightText(x, y, width, height) {
  highlights.value.push({ x, y, width, height });
}

export const showPdfFromUrl = async (filePath, page, positions) => {
  initialPageHighlights.value = { page, positions };
  console.log(initialPageHighlights.value);
  highlights.value = [];
  errorMessage.value = "";
  renderCanvas.value = false;
  try {
    // LoadPDF returns []byte which Wails serializes as base64; decode to Uint8Array
    const base64 = await LoadPDF(filePath);
    const binary = atob(base64);
    const uint8Array = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      uint8Array[i] = binary.charCodeAt(i);
    }
    renderCanvas.value = true;
    loadPdf(uint8Array, page, positions);
  } catch (error) {
    renderCanvas.value = false;
    throwError("❌ Fehler beim Laden der PDF-Datei", error);
  }
};
