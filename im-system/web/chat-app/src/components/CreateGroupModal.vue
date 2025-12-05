<template>
    <div
        class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
        @click.self="$emit('close')"
    >
        <div class="bg-white rounded-xl shadow-2xl w-full max-w-md mx-4">
            <!-- Header -->
            <div
                class="flex items-center justify-between p-4 border-b border-gray-200"
            >
                <h3 class="text-lg font-bold text-gray-900">创建群组</h3>
                <button
                    @click="$emit('close')"
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

            <!-- Form -->
            <form @submit.prevent="handleSubmit" class="p-4 space-y-4">
                <!-- Group Name -->
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                        群组名称 <span class="text-red-500">*</span>
                    </label>
                    <input
                        v-model="formData.name"
                        type="text"
                        required
                        class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                        placeholder="请输入群组名称"
                    />
                </div>

                <!-- Group Description -->
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                        群组描述
                    </label>
                    <textarea
                        v-model="formData.description"
                        rows="2"
                        class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition resize-none"
                        placeholder="请输入群组描述（可选）"
                    ></textarea>
                </div>

                <!-- Select Members -->
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                        邀请成员
                    </label>
                    <div
                        v-if="contacts.length > 0"
                        class="max-h-40 overflow-y-auto border border-gray-200 rounded-lg"
                    >
                        <div
                            v-for="contact in contacts"
                            :key="contact.id"
                            class="flex items-center p-2 hover:bg-gray-50 cursor-pointer"
                            @click="toggleMember(contact.id)"
                        >
                            <input
                                type="checkbox"
                                :checked="selectedMembers.includes(contact.id)"
                                class="w-4 h-4 text-blue-500 rounded border-gray-300 focus:ring-blue-500"
                                @click.stop
                                @change="toggleMember(contact.id)"
                            />
                            <div
                                class="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-medium mx-2"
                            >
                                {{ getAvatarText(contact) }}
                            </div>
                            <span class="text-sm text-gray-700">
                                {{ contact.nickname || contact.id }}
                            </span>
                        </div>
                    </div>
                    <p v-else class="text-sm text-gray-400 text-center py-4">
                        暂无联系人可邀请
                    </p>
                </div>

                <!-- Actions -->
                <div class="flex space-x-3 pt-2">
                    <button
                        type="button"
                        @click="$emit('close')"
                        class="flex-1 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg transition"
                    >
                        取消
                    </button>
                    <button
                        type="submit"
                        :disabled="!canSubmit"
                        class="flex-1 py-2 bg-blue-500 hover:bg-blue-600 text-white font-medium rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        创建
                    </button>
                </div>
            </form>
        </div>
    </div>
</template>

<script setup>
import { ref, computed } from "vue";

const props = defineProps({
    contacts: {
        type: Array,
        default: () => [],
    },
});

const emit = defineEmits(["close", "create"]);

const formData = ref({
    name: "",
    description: "",
});

const selectedMembers = ref([]);

const canSubmit = computed(() => {
    return formData.value.name.trim().length > 0;
});

function toggleMember(memberId) {
    const index = selectedMembers.value.indexOf(memberId);
    if (index > -1) {
        selectedMembers.value.splice(index, 1);
    } else {
        selectedMembers.value.push(memberId);
    }
}

function getAvatarText(contact) {
    const name = contact.nickname || contact.id || "";
    return name.charAt(0).toUpperCase();
}

function handleSubmit() {
    if (!canSubmit.value) return;

    emit("create", {
        name: formData.value.name.trim(),
        description: formData.value.description.trim(),
        memberIds: [...selectedMembers.value],
    });
}
</script>
