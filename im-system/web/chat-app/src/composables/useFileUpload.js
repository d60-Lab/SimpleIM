import { ref } from "vue";
import { useAuthStore } from "@/stores/auth";

export function useFileUpload() {
  const authStore = useAuthStore();

  const pendingFile = ref(null);
  const uploadProgress = ref(0);
  const isUploading = ref(false);

  function selectFile(file) {
    if (!file) return;

    // Validate file size (max 10MB)
    if (file.size > 10 * 1024 * 1024) {
      throw new Error("文件大小不能超过10MB");
    }

    pendingFile.value = {
      file,
      name: file.name,
      size: file.size,
      type: file.type,
      preview: null,
    };

    // Generate preview for images
    if (file.type.startsWith("image/")) {
      const reader = new FileReader();
      reader.onload = (e) => {
        if (pendingFile.value) {
          pendingFile.value.preview = e.target.result;
        }
      };
      reader.readAsDataURL(file);
    }

    return pendingFile.value;
  }

  function cancelUpload() {
    pendingFile.value = null;
    uploadProgress.value = 0;
  }

  async function uploadFile() {
    if (!pendingFile.value) {
      throw new Error("没有待上传的文件");
    }

    isUploading.value = true;
    uploadProgress.value = 0;

    const formData = new FormData();
    formData.append("file", pendingFile.value.file);

    try {
      const result = await new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener("progress", (e) => {
          if (e.lengthComputable) {
            uploadProgress.value = Math.round((e.loaded / e.total) * 100);
          }
        });

        xhr.addEventListener("load", () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            try {
              const resp = JSON.parse(xhr.responseText);
              resolve(resp);
            } catch {
              reject(new Error("解析响应失败"));
            }
          } else {
            reject(new Error(`上传失败: ${xhr.status}`));
          }
        });

        xhr.addEventListener("error", () => {
          reject(new Error("上传失败"));
        });

        xhr.open("POST", "/api/files/upload");
        xhr.setRequestHeader("Authorization", `Bearer ${authStore.token}`);
        xhr.send(formData);
      });

      // Clear pending file after successful upload
      const uploadedFile = pendingFile.value;
      pendingFile.value = null;
      uploadProgress.value = 0;

      return {
        ...result,
        originalName: uploadedFile.name,
        originalSize: uploadedFile.size,
        originalType: uploadedFile.type,
      };
    } finally {
      isUploading.value = false;
    }
  }

  function formatFileSize(bytes) {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  }

  function getFileExtension(filename) {
    const parts = filename.split(".");
    return parts.length > 1 ? parts.pop().toUpperCase() : "FILE";
  }

  function isImageFile(mimeType) {
    return mimeType && mimeType.startsWith("image/");
  }

  return {
    pendingFile,
    uploadProgress,
    isUploading,
    selectFile,
    cancelUpload,
    uploadFile,
    formatFileSize,
    getFileExtension,
    isImageFile,
  };
}
