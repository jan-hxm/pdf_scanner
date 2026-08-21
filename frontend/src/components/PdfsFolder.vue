<template>
  <div class="pdfs-folder">
    <p class="sidebar-label">PDF-Ordner</p>
    <p class="folder-path" :title="folder">{{ folder }}</p>
    <button
      type="button"
      class="btn-outline btn-block btn-start btn-sm"
      @click="changeFolder"
    >
      <span aria-hidden="true">📁</span>Ordner ändern
    </button>
  </div>
</template>

<script setup>
import { ref, onMounted } from "vue";
import { GetPDFsFolder, SelectPDFsFolder } from "../../wailsjs/go/main/App";
import { throwError } from "../composables/useError";

const folder = ref("");

const changeFolder = async () => {
  try {
    folder.value = await SelectPDFsFolder();
  } catch (error) {
    throwError("Ordner konnte nicht gesetzt werden", error);
  }
};

onMounted(async () => {
  folder.value = await GetPDFsFolder();
});
</script>
