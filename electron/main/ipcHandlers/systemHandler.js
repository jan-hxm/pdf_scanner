import fs from "fs";
import path from "path";
import { ipcMain } from "electron";
import { paths } from "../config";

export default function registerSystemHandlers() {
  ipcMain.handle("dialog:getPDFsFolder", () => paths.pdfFolder);

  ipcMain.handle("file:moveToPDFsFolder", async (event, filePath) => {
    const fileName = path.basename(filePath);
    const destinationPath = path.join(paths.pdfFolder, fileName);

    try {
      fs.copyFileSync(filePath, destinationPath);
      console.log(`PDF verschoben nach: ${destinationPath}`);
      return { success: true };
    } catch (err) {
      console.error(`Fehler beim Verschieben der Datei: ${err}`);
      return { success: false, error: err.message };
    }
  });
}
