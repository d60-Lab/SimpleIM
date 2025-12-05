<template>
    <div
        class="message-bubble rounded-lg px-4 py-2"
        :class="isFromMe ? 'bg-green-500 text-white' : 'bg-white shadow'"
    >
        <!-- Text Message -->
        <div v-if="contentType === 'text'" class="whitespace-pre-wrap break-words">
            {{ textContent }}
        </div>

        <!-- Image Message -->
        <div v-else-if="contentType === 'image'" class="max-w-xs">
            <img
                :src="imageUrl"
                :alt="fileName"
                class="rounded-lg cursor-pointer max-w-full h-auto"
                style="max-height: 200px"
                @click="$emit('imageClick', imageUrl)"
                @error="handleImageError"
            />
        </div>

        <!-- File Message -->
        <div
            v-else-if="contentType === 'file'"
            class="flex items-center space-x-3 p-2 rounded-lg"
            :class="isFromMe ? 'bg-green-600' : 'bg-gray-50'"
        >
            <div
                class="w-10 h-10 rounded flex items-center justify-center text-xs font-bold"
                :class="isFromMe ? 'bg-green-400 text-white' : 'bg-blue-100 text-blue-600'"
            >
                {{ fileExtension }}
            </div>
            <div class="flex-1 min-w-0">
                <p
                    class="text-sm font-medium truncate"
                    :class="isFromMe ? 'text-white' : 'text-gray-900'"
                >
                    {{ fileName }}
                </p>
                <p
                    class="text-xs"
                    :class="isFromMe ? 'text-green-100' : 'text-gray-500'"
                >
                    {{ formattedFileSize }}
                </p>
            </div>
            <a
                :href="fileUrl"
                target="_blank"
                download
                class="p-2 rounded-full transition"
                :class="
                    isFromMe
                        ? 'hover:bg-green-400 text-white'
                        : 'hover:bg-gray-200 text-gray-600'
                "
                @click.stop
            >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                    />
                </svg>
            </a>
        </div>

        <!-- Voice Message -->
        <div v-else-if="contentType === 'voice'" class="flex items-center space-x-2">
            <button
                class="p-2 rounded-full"
                :class="isFromMe ? 'bg-green-400 text-white' : 'bg-gray-200 text-gray-600'"
            >
                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z" />
                </svg>
            </button>
            <div class="w-24 h-1 rounded" :class="isFromMe ? 'bg-green-400' : 'bg-gray-300'"></div>
            <span class="text-xs" :class="isFromMe ? 'text-green-100' : 'text-gray-500'">
                {{ voiceDuration }}
            </span>
        </div>

        <!-- Video Message -->
        <div v-else-if="contentType === 'video'" class="max-w-xs">
            <video
                :src="videoUrl"
                controls
                class="rounded-lg max-w-full"
                style="max-height: 200px"
            >
                您的浏览器不支持视频播放
            </video>
        </div>

        <!-- Unknown Message Type -->
        <div v-else class="text-sm" :class="isFromMe ? 'text-green-100' : 'text-gray-500'">
            [不支持的消息类型]
        </div>
    </div>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
    message: {
        type: Object,
        required: true,
    },
    isFromMe: {
        type: Boolean,
        default: false,
    },
});

defineEmits(["imageClick"]);

const contentType = computed(() => {
    return props.message.contentType || "text";
});

const content = computed(() => {
    return props.message.content;
});

// Text content
const textContent = computed(() => {
    if (typeof content.value === "string") {
        return content.value;
    }
    if (content.value?.text) {
        return content.value.text;
    }
    return JSON.stringify(content.value);
});

// Image content
const imageUrl = computed(() => {
    if (typeof content.value === "object") {
        return content.value.url || content.value.thumbnail_url || "";
    }
    return "";
});

// File content
const fileName = computed(() => {
    if (typeof content.value === "object") {
        return content.value.file_name || "未知文件";
    }
    return "未知文件";
});

const fileSize = computed(() => {
    if (typeof content.value === "object") {
        return content.value.file_size || 0;
    }
    return 0;
});

const formattedFileSize = computed(() => {
    const bytes = fileSize.value;
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
});

const fileExtension = computed(() => {
    const name = fileName.value;
    const parts = name.split(".");
    if (parts.length > 1) {
        return parts.pop().toUpperCase().substring(0, 4);
    }
    return "FILE";
});

const fileUrl = computed(() => {
    if (typeof content.value === "object") {
        return content.value.url || "";
    }
    return "";
});

// Video content
const videoUrl = computed(() => {
    if (typeof content.value === "object") {
        return content.value.url || "";
    }
    return "";
});

// Voice content
const voiceDuration = computed(() => {
    if (typeof content.value === "object" && content.value.duration) {
        const seconds = Math.round(content.value.duration);
        return `${seconds}"`;
    }
    return "0\"";
});

function handleImageError(e) {
    e.target.src = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='100'%3E%3Crect fill='%23f3f4f6' width='100' height='100'/%3E%3Ctext fill='%239ca3af' x='50%25' y='50%25' text-anchor='middle' dy='.3em'%3E图片加载失败%3C/text%3E%3C/svg%3E";
}
</script>
