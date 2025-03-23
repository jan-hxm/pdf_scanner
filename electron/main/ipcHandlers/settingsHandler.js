import { paths } from "../config";
import { app, ipcMain } from "electron";
import fs from "fs";
import path from "path";

const settingsFilePath = paths.storage;

const defaultSettings = {
  theme: "light",
  language: "de",
  searchHistory: [],
  highlightColor: "#9B59B6",
};

// Funktion zum Laden der Einstellungen aus JSON
const loadSettings = () => {
  try {
    if (!fs.existsSync(settingsFilePath)) {
      fs.writeFileSync(
        settingsFilePath,
        JSON.stringify(defaultSettings, null, 2),
        "utf-8"
      );
    }
    const data = fs.readFileSync(settingsFilePath, "utf-8");
    return JSON.parse(data);
  } catch (error) {
    console.error("Fehler beim Laden der Einstellungen:", error);
    return defaultSettings;
  }
};

// Funktion zum Speichern von Einstellungen
const saveSettings = (newSettings) => {
  try {
    fs.writeFileSync(
      settingsFilePath,
      JSON.stringify(newSettings, null, 2),
      "utf-8"
    );
    return true;
  } catch (error) {
    console.error("Fehler beim Speichern der Einstellungen:", error);
    return false;
  }
};

// Electron IPC Handler registrieren
export default function registerSettingsHandlers() {
  console.log("🔧 Register settings handlers");

  // Rückgabe des Speicherpfads
  ipcMain.handle("dialog:getStorage", async () => {
    return settingsFilePath;
  });

  // Einstellungen aus der Datei lesen
  ipcMain.handle("settings:load", async () => {
    return loadSettings();
  });

  // Einstellungen speichern (mit Merge der neuen Werte)
  ipcMain.handle("settings:save", async (event, newSettings) => {
    const currentSettings = loadSettings();
    const updatedSettings = { ...currentSettings, ...newSettings }; // Merge mit bestehenden Einstellungen
    return saveSettings(updatedSettings);
  });
}
