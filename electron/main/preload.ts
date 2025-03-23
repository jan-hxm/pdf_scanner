const { ipcRenderer, contextBridge } = require("electron");

console.log("✅ Preload-Skript wurde geladen!");

contextBridge.exposeInMainWorld("ipcRenderer", {
  on(channel: string, listener: (event: any, ...args: any[]) => void) {
    ipcRenderer.removeAllListeners(channel);
    ipcRenderer.on(channel, (event, ...args) => listener(event, ...args));
  },
  off(channel: string, listener: (...args: any[]) => void) {
    ipcRenderer.off(channel, listener);
  },
  send(channel: string, ...args: any[]) {
    return ipcRenderer.send(channel, ...args);
  },
  invoke(channel: string, ...args: any[]) {
    return ipcRenderer.invoke(channel, ...args);
  },
});
