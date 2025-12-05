<template>
    <div>
        <!-- Group Header -->
        <div class="text-center pb-4 border-b border-gray-200">
            <div
                class="w-16 h-16 rounded-full bg-green-500 flex items-center justify-center text-white text-2xl font-bold mx-auto mb-3"
            >
                {{ groupAvatarText }}
            </div>
            <h3 class="font-bold text-gray-900">{{ group?.name || "群组" }}</h3>
            <p class="text-sm text-gray-500 mt-1">{{ group?.description || "" }}</p>
            <div class="flex items-center justify-center mt-2 space-x-2">
                <span class="text-xs text-gray-400">群ID:</span>
                <span class="text-xs text-gray-600 font-mono">{{ shortGroupId }}</span>
                <button
                    @click="$emit('copyId', groupId)"
                    class="text-blue-500 hover:text-blue-600 text-xs"
                >
                    复制
                </button>
            </div>
        </div>

        <!-- Members Section -->
        <div class="mt-4">
            <h4 class="text-sm font-medium text-gray-700 mb-3">
                成员 ({{ members.length }})
            </h4>
            <div class="space-y-2 max-h-64 overflow-y-auto scrollbar-thin">
                <div
                    v-for="member in members"
                    :key="member.user_id"
                    class="flex items-center p-2 rounded-lg hover:bg-gray-50"
                >
                    <div
                        class="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-medium mr-3"
                        :class="getMemberAvatarClass(member)"
                    >
                        {{ getMemberAvatarText(member) }}
                    </div>
                    <div class="flex-1 min-w-0">
                        <p class="text-sm font-medium text-gray-900 truncate">
                            {{ member.nickname || member.user_id }}
                        </p>
                        <p class="text-xs text-gray-500">
                            {{ member.user_id }}
                        </p>
                    </div>
                    <span
                        v-if="member.role"
                        class="text-xs px-2 py-0.5 rounded"
                        :class="getRoleBadgeClass(member.role)"
                    >
                        {{ getRoleText(member.role) }}
                    </span>
                </div>
            </div>
        </div>

        <!-- Actions -->
        <div class="mt-6 pt-4 border-t border-gray-200">
            <button
                @click="$emit('leave')"
                class="w-full py-2 bg-red-50 hover:bg-red-100 text-red-600 rounded-lg transition text-sm font-medium"
            >
                退出群组
            </button>
        </div>
    </div>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
    group: {
        type: Object,
        default: null,
    },
    members: {
        type: Array,
        default: () => [],
    },
    currentUserId: {
        type: String,
        default: "",
    },
});

defineEmits(["leave", "copyId"]);

const groupId = computed(() => {
    return props.group?.group_id || props.group?.id || "";
});

const shortGroupId = computed(() => {
    const id = groupId.value;
    if (id && id.length > 12) {
        return id.substring(0, 6) + "..." + id.substring(id.length - 6);
    }
    return id;
});

const groupAvatarText = computed(() => {
    const name = props.group?.name || "";
    return name.charAt(0).toUpperCase() || "G";
});

function getMemberAvatarText(member) {
    const name = member.nickname || member.user_id || "";
    return name.charAt(0).toUpperCase() || "?";
}

function getMemberAvatarClass(member) {
    if (member.role === "owner") return "bg-yellow-500";
    if (member.role === "admin") return "bg-blue-500";
    return "bg-gray-400";
}

function getRoleText(role) {
    switch (role) {
        case "owner":
            return "群主";
        case "admin":
            return "管理员";
        default:
            return "";
    }
}

function getRoleBadgeClass(role) {
    switch (role) {
        case "owner":
            return "bg-yellow-100 text-yellow-700";
        case "admin":
            return "bg-blue-100 text-blue-700";
        default:
            return "bg-gray-100 text-gray-600";
    }
}
</script>
