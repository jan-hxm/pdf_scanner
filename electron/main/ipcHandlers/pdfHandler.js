import { dialog, ipcMain } from "electron";
import { spawn } from "child_process";
import { paths } from "../config";
import runScript from "./pythonHandler.js";
import fs from "fs";

export default function registerPDFHandlers() {
  console.log("Event-Handler für 'start-pdf-search' wird registriert...");
  ipcMain.on("start-pdf-search", async (event, keyword) => {
    try {
      if (!paths.pdfScannerPath || !paths.pdfFolder) {
        throw new Error("Fehlende Pfade für PDF-Suche!");
      }

      console.log("📌 Suche nach:", keyword);

      const pythonProcess = spawn(paths.pythonExecutable, [
        paths.pdfScannerPath,
        paths.pdfFolder,
        keyword,
      ]);

      let outputData = "";

      pythonProcess.stdout.on("data", (data) => {
        outputData += data.toString();

        const lines = outputData.split("\n");

        for (let i = 0; i < lines.length - 1; i++) {
          if (lines[i].trim()) {
            try {
              const result = JSON.parse(lines[i].trim());
              event.reply("pdf-search-progress", result);
            } catch (e) {
              console.error("⚠️ JSON Parsing Error:", e, "Zeile:", lines[i]);
            }
          }
        }

        outputData = lines[lines.length - 1];
      });

      pythonProcess.stderr.on("data", (data) => {
        console.error(`Error: ${data.toString()}`);
        event.reply("pdf-search-error", data.toString());
      });

      pythonProcess.on("close", (code) => {
        if (code !== 0) {
          console.error(`Python process exited with code ${code}`);
          event.reply(
            "pdf-search-error",
            `Python process exited with code ${code}`
          );
        } else {
          console.log(`Python process beendet (Code: ${code})`);
        }
      });
    } catch (error) {
      console.error("⚠️ PDF-Suche Fehler:", error);
      event.reply("pdf-search-error", error.toString());
    }
  });

  ipcMain.handle("pdf:openPdf", async (event, pdfFilePath, pageNumber) => {
    try {
      console.log("📌 Öffne PDF:", pdfFilePath, "auf Seite", pageNumber);

      const result = await runScript("pdfOpenerPath", [
        pdfFilePath,
        pageNumber,
      ]);

      console.log("📄 PDF-Öffnungsergebnis:", result);
      return result;
    } catch (error) {
      console.error("⚠️ Fehler beim Öffnen des PDFs:", error);
      throw error;
    }
  });

  ipcMain.handle("pdf:selectFile", async () => {
    const result = await dialog.showOpenDialog({
      properties: ["openFile"],
      filters: [{ name: "PDF", extensions: ["pdf"] }],
    });

    if (result.canceled) {
      return null;
    }

    return result.filePaths[0];
  });

  ipcMain.handle("dialog:selectPDFs", async () => {
    const { canceled, filePaths } = await dialog.showOpenDialog({
      properties: ["openFile", "multiSelections"],
      filters: [{ name: "PDFs", extensions: ["pdf"] }],
    });

    if (!canceled && filePaths.length > 0) {
      return filePaths;
    }
    return null;
  });

  ipcMain.handle("pdf:load-pdf", async (event, filePath) => {
    try {
      const data = fs.readFileSync(filePath); // Datei synchron lesen
      return new Uint8Array(data); // Als Uint8Array an Renderer senden
    } catch (error) {
      console.error("❌ Fehler beim Laden der PDF-Datei:", error);
      throw error;
    }
  });
}
