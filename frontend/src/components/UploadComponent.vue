<template>
  <div class="upload-component">
    <button
      class="btn-outline upload-btn"
      @click="uploadActive = !uploadActive"
    >
      📄 Neue PDF-Datei hinzufügen
    </button>
    <transition name="top">
      <div class="upload-container" v-if="uploadActive">
        <div
          class="drop-area"
          @click="selectFiles"
          @dragover.prevent
          @drop="handleFileDrop"
        >
          <h2>Anklicken oder PDF-Dateien hierher ziehen</h2>

          <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
          <p v-if="successMessage" class="success-message">
            {{ successMessage }}
          </p>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, onMounted } from "vue";
import { SelectFiles, MoveFileToPDFsFolder, GetPDFsFolder } from "../../wailsjs/go/main/App";

const errorMessage = ref("");
const successMessage = ref("");
const uploadActive = ref(false);
const uploadFolder = ref("");

const selectFiles = async () => {
  console.log(uploadFolder.value);
  const filePaths = await SelectFiles();
  if (filePaths && filePaths.length > 0) {
    for (const filePath of filePaths) {
      if (filePath.endsWith(".pdf")) {
        try {
          await MoveFileToPDFsFolder(filePath);
          successMessage.value = `PDF erfolgreich nach ${uploadFolder.value} verschoben.`;
          errorMessage.value = null;
        } catch (err) {
          console.log(err);
          errorMessage.value = `Fehler: ${err}`;
          successMessage.value = null;
        }
      } else {
        alert("Bitte nur PDF-Dateien auswählen!");
      }
    }
  }
};

const handleFileDrop = async (event) => {
  event.preventDefault();
  const { files } = event.dataTransfer;

  for (let i = 0; i < files.length; i++) {
    const filePath = files[i].path;
    if (filePath.endsWith(".pdf")) {
      try {
        await MoveFileToPDFsFolder(filePath);
        successMessage.value = `${filePath} erfolgreich gespeichert.`;
        errorMessage.value = null;
      } catch (err) {
        errorMessage.value = `Fehler: ${err}`;
        successMessage.value = null;
      }
    } else {
      alert("Bitte nur PDF-Dateien verschieben");
    }
  }
};

onMounted(() => {
  GetPDFsFolder().then((res) => {
    uploadFolder.value = res;
  });
});
</script>
