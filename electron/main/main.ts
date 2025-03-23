import { app, nativeTheme, protocol, BrowserWindow } from "electron";
import registerPDFHandlers from "./ipcHandlers/pdfHandler";
import registerSystemHandlers from "./ipcHandlers/systemHandler";
import registerSettingsHandlers from "./ipcHandlers/settingsHandler";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isDev = !app.isPackaged;
let win: BrowserWindow | null;

protocol.registerSchemesAsPrivileged([
  { scheme: "app", privileges: { secure: true, standard: true } },
]);

app.whenReady().then(() => {
  console.log("Electron ist bereit");
  nativeTheme.themeSource = "light";

  const preloadPath = path.join(__dirname, "../dist-electron/preload.mjs");

  registerPDFHandlers();
  registerSystemHandlers();
  registerSettingsHandlers();

  win = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 680,
    minHeight: 580,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: preloadPath,
    },
  });

  if (isDev) {
    win.loadURL("http://localhost:5173");
    win.webContents.openDevTools();
  } else {
    win
      .loadFile(path.join(app.getAppPath(), "dist/index.html"))
      .catch((err) => {
        console.error("Fehler beim Laden der App-URL:", err);
      });
  }

  win.on("closed", () => {
    win = null;
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
