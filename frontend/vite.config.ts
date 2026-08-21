import { defineConfig } from 'vite'
import fs from 'node:fs'
import path from 'node:path'
import vue from '@vitejs/plugin-vue'

// Single source for the version: wails.json's info.productVersion, which is
// also what stamps the .exe's file metadata. Read here so the UI cannot drift
// from the binary.
const { info } = JSON.parse(
  fs.readFileSync(path.resolve(__dirname, '../wails.json'), 'utf-8')
)

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  define: {
    __APP_VERSION__: JSON.stringify(info.productVersion),
  },
  plugins: [vue()],
})
