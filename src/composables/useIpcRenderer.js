export function useIpcRenderer() {
  return window.ipcRenderer ? window.ipcRenderer : null;
}
