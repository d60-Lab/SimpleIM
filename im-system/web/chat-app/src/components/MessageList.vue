<template>
    <div
        ref="containerRef"
        class="flex-1 overflow-y-auto p-4 space-y-4 scrollbar-thin"
    >
        <div
            v-if="messages.length === 0"
            class="flex items-center justify-center h-full text-gray-400"
        >
            <div class="text-center">
                <div class="text-4xl mb-2">💬</div>
                <p>暂无消息</p>
            </div>
        </div>

        <div
            v-for="message in messages"
            :key="message.id"
            class="flex animate-fade-in"
            :class="message.isFromMe ? 'justify-end' : 'justify-start'"
        >
            <!-- Other user's message -->
            <div
                v-if="!message.isFromMe"
                class="flex items-start max-w-[70%]"
            >
                <div
                    class="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-medium mr-2 flex-shrink-0"
                >
                    {{ getAvatarText(message.from) }}
                </div>
                <div>
                    <div class="text-xs text-gray-500 mb-1">
                        {{ message.from }}
                        <span class="ml-2">{{ formatTime(message.timestamp) }}</span>
                    </div>
                    <MessageContent
                        :message="message"
                        :is-from-me="false"
                        @image-click="$emit('imageClick', $event)"
                    />
                </div>
            </div>

            <!-- My message -->
            <div v-else class="flex items-start max-w-[70%] flex-row-reverse">
                <div
                    class="w-8 h-8 rounded-full bg-green-500 flex items-center justify-center text-white text-sm font-medium ml-2 flex-shrink-0"
                >
                    {{ getAvatarText(currentUserId) }}
                </div>
                <div class="text-right">
                    <div class="text-xs text-gray-500 mb-1">
                        {{ formatTime(message.timestamp) }}
                    </div>
                    <MessageContent
                        :message="message"
                        :is-from-me="true"
                        @image-click="$emit('imageClick', $event)"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, nextTick } from "vue";
import MessageContent from "./MessageContent.vue";

const props = defineProps({
    messages: {
        type: Array,
        required: true,
    },
    currentUserId: {
        type: String,
        required: true,
    },
});

defineEmits(["imageClick"]);

const containerRef = ref(null);

function scrollToBottom() {
    nextTick(() => {
        if (containerRef.value) {
            containerRef.value.scrollTop = containerRef.value.scrollHeight;
        }
    });
}

function getAvatarText(userId) {
    if (!userId) return "?";
    return userId.charAt(0).toUpperCase();
}

function formatTime(timestamp) {
    if (!timestamp) return "";
    const date = new Date(timestamp);
    return date.toLocaleTimeString("zh-CN", {
        hour: "2-digit",
        minute: "2-digit",
    });
}

// Expose methods to parent
defineExpose({
    scrollToBottom,
});
</script>
