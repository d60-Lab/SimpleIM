<template>
    <div class="space-y-1">
        <div
            v-if="contacts.length === 0"
            class="text-center text-gray-400 py-8"
        >
            <p class="text-sm">暂无联系人</p>
            <p class="text-xs mt-1">输入用户ID开始聊天</p>
        </div>
        <div
            v-for="contact in contacts"
            :key="contact.id"
            @click="$emit('select', contact.id)"
            class="flex items-center p-3 rounded-lg cursor-pointer transition"
            :class="
                isActive(contact.id)
                    ? 'bg-blue-50 border border-blue-200'
                    : 'hover:bg-gray-50'
            "
        >
            <!-- Avatar -->
            <div
                class="w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-medium mr-3 flex-shrink-0"
            >
                {{ getAvatarText(contact) }}
            </div>

            <!-- Info -->
            <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between">
                    <span class="font-medium text-gray-900 truncate">
                        {{ contact.nickname || contact.id }}
                    </span>
                    <span
                        v-if="getUnreadCount('user', contact.id) > 0"
                        class="ml-2 px-2 py-0.5 bg-red-500 text-white text-xs rounded-full flex-shrink-0"
                    >
                        {{ getUnreadCount("user", contact.id) }}
                    </span>
                </div>
                <p class="text-sm text-gray-500 truncate">
                    {{ getMessagePreview(`user:${contact.id}`) }}
                </p>
            </div>
        </div>
    </div>
</template>

<script setup>
const props = defineProps({
    contacts: {
        type: Array,
        required: true,
    },
    currentChat: {
        type: Object,
        default: null,
    },
    getUnreadCount: {
        type: Function,
        required: true,
    },
    getMessagePreview: {
        type: Function,
        required: true,
    },
});

defineEmits(["select"]);

function isActive(contactId) {
    return (
        props.currentChat?.type === "user" && props.currentChat?.id === contactId
    );
}

function getAvatarText(contact) {
    const name = contact.nickname || contact.id || "";
    return name.charAt(0).toUpperCase();
}
</script>
