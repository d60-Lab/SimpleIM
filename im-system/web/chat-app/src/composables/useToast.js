import { ref } from "vue";

const toasts = ref([]);
let toastId = 0;

export function useToast() {
  function showToast(message, type = "info", duration = 3000) {
    const id = ++toastId;

    toasts.value.push({
      id,
      message,
      type, // 'info' | 'success' | 'error' | 'warning'
    });

    setTimeout(() => {
      removeToast(id);
    }, duration);

    return id;
  }

  function removeToast(id) {
    const index = toasts.value.findIndex((t) => t.id === id);
    if (index > -1) {
      toasts.value.splice(index, 1);
    }
  }

  function success(message, duration = 3000) {
    return showToast(message, "success", duration);
  }

  function error(message, duration = 3000) {
    return showToast(message, "error", duration);
  }

  function warning(message, duration = 3000) {
    return showToast(message, "warning", duration);
  }

  function info(message, duration = 3000) {
    return showToast(message, "info", duration);
  }

  return {
    toasts,
    showToast,
    removeToast,
    success,
    error,
    warning,
    info,
  };
}
