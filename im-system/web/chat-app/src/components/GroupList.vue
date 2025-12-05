<template>
    <div class="space-y-1">
        <div v-if="groups.length === 0" class="text-center text-gray-400 py-8">
            <p class="text-sm">暂无群组</p>
            <p class="text-xs mt-1">创建或加入一个群组</p>
        </div>
        <div
            v-for="group in groups"
            :key="getGroupId(group)"
            @click="$emit('select', getGroupId(group))"
            class="flex items-center p-3 rounded-lg cursor-pointer transition"
            :class="
                isActive(group)
                    ? 'bg-blue-50 border border-blue-200'
                    : 'hover:bg-gray-50'
            "
        >
            <!-- Avatar -->
            <div
                class="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center text-white font-medium mr-3 flex-shrink-0"
            >
                {{ getAvatarText(group) }}
            </div>

            <!-- Info -->
            <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between">
                    <div class="flex items-center min-w-0">
                        <span class="font-medium text-gray-900 truncate">
                            {{ group.name || getGroupId(group) }}
                        </span>
                        <span class="ml-2 text-xs text-gray-400 flex-shrink-0">
                            #{{ getShortGroupId(group) }}
                        </span>
                    </div>
                    <div class="flex items-center flex-shrink-0 ml-2">
                        <span
                            v-if="getUnreadCount('group', getGroupId(group)) > 0"
                            class="px-2 py-0.5 bg-red-500 text-white text-xs rounded-full"
                        >
                            {{ getUnreadCount("group", getGroupId(group)) }}
                        </span>
                        <span v-else class="text-xs text-gray-400">
                            {{ group.member_count || 0 }}人
                        </span>
                    </div>
                </div>
                <p class="text-sm text-gray-500 truncate">
                    {{ getMessagePreview(`group:${getGroupId(group)}`) }}
                </p>
            </div>
        </div>
    </div>
</template>

<script setup>
const props = defineProps({
    groups: {
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

function getGroupId(group) {
    return group.group_id || group.id;
}

function getShortGroupId(group) {
    const id = getGroupId(group);
    if (id && id.length > 8) {
        return id.substring(id.length - 8);
    }
    return id;
}

function isActive(group) {
    return (
        props.currentChat?.type === "group" &&
        props.currentChat?.id === getGroupId(group)
    );
}

function getAvatarText(group) {
    const name = group.name || getGroupId(group) || "";
    return name.charAt(0).toUpperCase();
}
</script>
