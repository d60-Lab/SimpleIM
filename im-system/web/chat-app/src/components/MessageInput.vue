<template>
    <div>
        <!-- File Preview -->
        <div
            v-if="pendingFile"
            class="mb-3 p-3 bg-gray-50 rounded-lg border border-gray-200"
        >
            <div class="flex items-center justify-between">
                <div class="flex items-center space-x-3">
                    <!-- Image Preview -->
                    <div v-if="pendingFile.preview" class="relative">
                        <img
                            :src="pendingFile.preview"
                            class="w-16 h-16 object-cover rounded"
                            alt="Preview"
                        />
                    </div>
                    <!-- File Icon -->
                    <div
                        v-else
                        class="w-16 h-16 bg-blue-100 rounded flex items-center justify-center"
                    >
                        <span class="text-blue-600 font-bold text-sm">
                            {{ getFileExtension(pendingFile.name) }}
                        </span>
                    </div>

                    <!-- File Info -->
                    <div>
                        <p
                            class="text-sm font-medium text-gray-900 truncate max-w-xs"
                        >
                            {{ pendingFile.name }}
                        </p>
                        <p class="text-xs text-gray-500">
                            {{ formatFileSize(pendingFile.size) }}
                        </p>
                    </div>
                </div>

                <!-- Actions -->
                <button
                    @click="$emit('cancelFile')"
                    class="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-full transition"
                >
                    <svg
                        class="w-5 h-5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M6 18L18 6M6 6l12 12"
                        />
                    </svg>
                </button>
            </div>

            <!-- Upload Progress -->
            <div v-if="isUploading" class="mt-2">
                <div class="h-1 bg-gray-200 rounded-full overflow-hidden">
                    <div
                        class="h-full bg-blue-500 transition-all duration-300"
                        :style="{ width: `${uploadProgress}%` }"
                    ></div>
                </div>
                <p class="text-xs text-gray-500 mt-1 text-center">
                    上传中... {{ uploadProgress }}%
                </p>
            </div>

            <!-- Send File Button -->
            <div v-else class="mt-3 flex justify-end">
                <button
                    @click="$emit('sendFile')"
                    class="px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white text-sm rounded-lg transition"
                >
                    发送文件
                </button>
            </div>
        </div>

        <!-- Input Area -->
        <div class="flex items-end space-x-3">
            <!-- File Upload Button -->
            <label
                class="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-full cursor-pointer transition"
            >
                <input type="file" class="hidden" @change="handleFileChange" />
                <svg
                    class="w-6 h-6"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
                    />
                </svg>
            </label>

            <!-- Text Input -->
            <div class="flex-1 relative">
                <textarea
                    ref="textareaRef"
                    v-model="inputText"
                    @keydown="handleKeyDown"
                    @input="adjustHeight"
                    rows="1"
                    class="w-full px-4 py-3 border border-gray-300 rounded-xl resize-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                    style="max-height: 128px"
                    placeholder="输入消息..."
                ></textarea>
            </div>

            <!-- Send Button -->
            <button
                @click="sendMessage"
                :disabled="!canSend"
                class="p-3 bg-blue-500 hover:bg-blue-600 text-white rounded-xl transition disabled:opacity-50 disabled:cursor-not-allowed"
            >
                <svg
                    class="w-6 h-6"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
                    />
                </svg>
            </button>
        </div>
    </div>
</template>

<script setup>
import { ref, computed, nextTick } from "vue";

const props = defineProps({
    pendingFile: {
        type: Object,
        default: null,
    },
    uploadProgress: {
        type: Number,
        default: 0,
    },
    isUploading: {
        type: Boolean,
        default: false,
    },
});

const emit = defineEmits(["send", "fileSelect", "cancelFile", "sendFile"]);

const inputText = ref("");
const textareaRef = ref(null);

const canSend = computed(() => {
    return inputText.value.trim().length > 0;
});

function sendMessage() {
    const text = inputText.value.trim();
    if (!text) return;

    emit("send", text);
    inputText.value = "";

    nextTick(() => {
        adjustHeight();
    });
}

function handleKeyDown(e) {
    // Enter to send, Shift+Enter for new line
    if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
    }
}

function adjustHeight() {
    nextTick(() => {
        if (textareaRef.value) {
            textareaRef.value.style.height = "auto";
            textareaRef.value.style.height =
                Math.min(textareaRef.value.scrollHeight, 128) + "px";
        }
    });
}

function handleFileChange(e) {
    const file = e.target.files?.[0];
    if (file) {
        emit("fileSelect", file);
    }
    // Reset input so same file can be selected again
    e.target.value = "";
}

function formatFileSize(bytes) {
    if (!bytes) return "0 B";
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

function getFileExtension(filename) {
    if (!filename) return "FILE";
    const parts = filename.split(".");
    if (parts.length > 1) {
        return parts.pop().toUpperCase().substring(0, 4);
    }
    return "FILE";
}
</script>
