import { ref } from "vue";

export const errorMessage = ref("");

// Function to create and show a toast message
const showToast = (message) => {
  const toast = document.createElement("div");
  toast.textContent = message;
  toast.className = "custom-toast";

  document.body.appendChild(toast);

  // Fade the toast out after 2 seconds, then drop it from the DOM
  setTimeout(() => {
    toast.classList.add("hide");
    setTimeout(() => toast.remove(), 500);
  }, 2000);
};

/**
 * Normalises whatever the caller caught into a readable string. Wails rejects
 * a bound method's promise with the Go error's message, but a thrown JS Error
 * arrives as an object.
 */
const describe = (errorObj) => {
  if (!errorObj) return "";
  if (typeof errorObj === "string") return errorObj;
  return errorObj.message || String(errorObj);
};

export const throwError = (msg, errorObj = null) => {
  if (!msg) return;

  const detail = describe(errorObj);
  const full = detail ? `${msg}: ${detail}` : msg;

  errorObj ? console.error(msg, errorObj) : console.error(msg);
  errorMessage.value = full;
  showToast(full);
};

export const clearError = () => {
  errorMessage.value = "";
};
