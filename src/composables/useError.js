import { ref } from "vue";

export const errorMessage = ref("");

// Function to create and show a toast message
const showToast = (message) => {
  const toast = document.createElement("div");
  toast.textContent = message;
  toast.className = "custom-toast";

  document.body.appendChild(toast);

  // Automatically remove toast after 5 seconds
  setTimeout(() => {
    toast.classList.add("hide");
    setTimeout(() => toast.remove(), 500);
  }, 2000);
};

export const throwError = (msg, errorObj = null) => {
  if (msg) {
    errorObj ? console.error(msg, errorObj) : console.error(msg);
    showToast(errorObj ? `${msg}: ${errorObj}` : msg);
  }
};
