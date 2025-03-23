import path from "path";
import fs from "fs";
import { app } from "electron";

const isDevelopment = !app.isPackaged;

const getBaseFolder = (): string =>
  isDevelopment
    ? "C:\\Users\\janhe\\Documents\\GitHub\\pdf-searcher\\electron\\resources"
    : path.join(process.resourcesPath);

const getFileFromWorkingDir = (fileName: string) => {
  const filePath = path.join(getBaseFolder(), fileName);
  try {
    if (fs.existsSync(filePath)) {
      const fileData = JSON.parse(fs.readFileSync(filePath, "utf-8"));
      return fileData;
    }
  } catch (error) {
    console.error(`⚠️ Fehler beim Lesen von ${fileName}:`, error);
  }
};

const getWorkingDir = () => {
  const configData = getFileFromWorkingDir("production_workdir.json");
  try {
    return configData.workdir;
  } catch (error) {
    console.error("⚠️ get working dir file error:", error);
  }
};

export const paths = {
  pythonExecutable: path.join(
    getWorkingDir(),
    "WPy64-31131",
    "python-3.11.3.amd64",
    "python.exe"
  ),
  baseFolder: getBaseFolder(),
  storage: path.join(getWorkingDir(), "search_history.json"),
  pdfFolder: path.join(getWorkingDir(), "pdf"),
  pdfScannerPath: path.join(getBaseFolder(), "python", "reader.py"),
  pdfOpenerPath: path.join(getBaseFolder(), "python", "highlight_search.py"),
};
