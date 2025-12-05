<template>
    <div class="h-screen flex bg-gray-100">
        <!-- Sidebar -->
        <div class="w-80 bg-white border-r border-gray-200 flex flex-col">
            <!-- Header -->
            <div
                class="h-16 bg-blue-500 flex items-center justify-between px-4"
            >
                <h2 class="text-white font-bold text-lg flex items-center">
                    <span class="mr-2">💬</span> 聊天
                </h2>
                <div class="flex items-center space-x-3">
                    <div class="flex items-center">
                        <span
                            class="w-2 h-2 rounded-full mr-2"
                            :class="{
                                'bg-green-400': connectionStatus === 'connected',
                                'bg-yellow-400 animate-pulse':
                                    connectionStatus === 'connecting',
                                'bg-red-400': connectionStatus === 'disconnected',
                            }"
                        ></span>
                        <span class="text-white text-sm">{{
                            connectionStatusText
                        }}</span>
                    </div>
                    <span class="text-white text-sm">{{ authStore.nickname }}</span>
                    <button
                        @click="handleLogout"
                        class="px-3 py-1 bg-white/20 hover:bg-white/30 text-white text-sm rounded transition"
                    >
                        退出
                    </button>
                </div>
            </div>

            <!-- Tabs -->
            <div class="flex border-b border-gray-200">
                <button
                    @click="activeTab = 'contacts'"
                    class="flex-1 py-3 text-center transition"
                    :class="
                        activeTab === 'contacts'
                            ? 'text-blue-500 border-b-2 border-blue-500 font-medium'
                            : 'text-gray-500 hover:text-gray-700'
                    "
                >
                    👤 联系人
                </button>
                <button
                    @click="activeTab = 'groups'"
                    class="flex-1 py-3 text-center transition"
                    :class="
                        activeTab === 'groups'
                            ? 'text-blue-500 border-b-2 border-blue-500 font-medium'
                            : 'text-gray-500 hover:text-gray-700'
                    "
                >
                    👥 群组
                </button>
            </div>

            <!-- Content -->
            <div class="flex-1 overflow-y-auto scrollbar-thin">
                <!-- Contacts Tab -->
                <div v-if="activeTab === 'contacts'" class="p-3">
                    <div class="mb-3">
                        <input
                            v-model="newContactId"
                            @keydown.enter="startUserChat(newContactId)"
                            type="text"
                            placeholder="输入用户ID开始聊天"
                            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                    </div>
                    <ContactList
                        :contacts="chatStore.contacts"
                        :current-chat="chatStore.currentChat"
                        :get-unread-count="chatStore.getUnreadCount"
                        :get-message-preview="chatStore.getMessagePreview"
                        @select="startUserChat"
                    />
                </div>

                <!-- Groups Tab -->
                <div v-if="activeTab === 'groups'" class="p-3">
                    <button
                        @click="showCreateGroupModal = true"
                        class="w-full mb-3 py-2 bg-green-500 hover:bg-green-600 text-white rounded-lg transition flex items-center justify-center"
                    >
                        <span class="mr-1">+</span> 创建群组
                    </button>
                    <div class="mb-3">
                        <input
                            v-model="joinGroupId"
                            @keydown.enter="handleJoinGroup"
                            type="text"
                            placeholder="输入群ID加入群组"
                            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                    </div>
                    <GroupList
                        :groups="chatStore.groups"
                        :current-chat="chatStore.currentChat"
                        :get-unread-count="chatStore.getUnreadCount"
                        :get-message-preview="chatStore.getMessagePreview"
                        @select="startGroupChat"
                    />
                </div>
            </div>
        </div>

        <!-- Chat Area -->
        <div class="flex-1 flex flex-col">
            <!-- Chat Header -->
            <div
                v-if="chatStore.currentChat"
                class="h-16 bg-white border-b border-gray-200 flex items-center justify-between px-6"
            >
                <div class="flex items-center">
                    <span class="mr-2">{{
                        chatStore.currentChat.type === "group" ? "👥" : "👤"
                    }}</span>
                    <span class="font-medium">{{ currentChatName }}</span>
                </div>
                <div v-if="chatStore.currentChat.type === 'group'">
                    <button
                        @click="showGroupInfo = !showGroupInfo"
                        class="px-3 py-1 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded transition"
                    >
                        ℹ️ 详情
                    </button>
                </div>
            </div>

            <!-- Messages -->
            <div class="flex-1 overflow-hidden flex">
                <div
                    v-if="!chatStore.currentChat"
                    class="flex-1 flex items-center justify-center text-gray-400"
                >
                    <div class="text-center">
                        <div class="text-6xl mb-4">💬</div>
                        <p>开始聊天吧！</p>
                    </div>
                </div>
                <MessageList
                    v-else
                    ref="messageListRef"
                    :messages="chatStore.currentMessages"
                    :current-user-id="authStore.userId"
                    @image-click="showImagePreview"
                />

                <!-- Group Info Panel -->
                <div
                    v-if="showGroupInfo && chatStore.currentChat?.type === 'group'"
                    class="w-72 bg-white border-l border-gray-200 p-4 overflow-y-auto scrollbar-thin"
                >
                    <GroupInfoPanel
                        :group="chatStore.currentGroup"
                        :members="chatStore.currentGroupMembers"
                        :current-user-id="authStore.userId"
                        @leave="handleLeaveGroup"
                        @copy-id="copyGroupId"
                    />
                </div>
            </div>

            <!-- Input Area -->
            <div v-if="chatStore.currentChat" class="bg-white border-t border-gray-200 p-4">
                <MessageInput
                    :pending-file="fileUpload.pendingFile.value"
                    :upload-progress="fileUpload.uploadProgress.value"
                    :is-uploading="fileUpload.isUploading.value"
                    @send="handleSendMessage"
                    @file-select="handleFileSelect"
                    @cancel-file="fileUpload.cancelUpload"
                    @send-file="handleSendFile"
                />
            </div>
        </div>

        <!-- Create Group Modal -->
        <CreateGroupModal
            v-if="showCreateGroupModal"
            :contacts="chatStore.contacts"
            @close="showCreateGroupModal = false"
            @create="handleCreateGroup"
        />

        <!-- Image Preview Modal -->
        <div
            v-if="previewImage"
            class="image-preview-overlay"
            @click="previewImage = null"
        >
            <img :src="previewImage" alt="Preview" />
        </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useChatStore } from "@/stores/chat";
import { useWebSocket } from "@/composables/useWebSocket";
import { useFileUpload } from "@/composables/useFileUpload";
import { useToast } from "@/composables/useToast";

import ContactList from "@/components/ContactList.vue";
import GroupList from "@/components/GroupList.vue";
import MessageList from "@/components/MessageList.vue";
import MessageInput from "@/components/MessageInput.vue";
import GroupInfoPanel from "@/components/GroupInfoPanel.vue";
import CreateGroupModal from "@/components/CreateGroupModal.vue";

const router = useRouter();
const authStore = useAuthStore();
const chatStore = useChatStore();
const toast = useToast();

const {
    connectionStatus,
    connect,
    disconnect,
    sendPrivateMessage,
    sendGroupMessage,
    sendFileMessage,
} = useWebSocket();

const fileUpload = useFileUpload();

const activeTab = ref("contacts");
const newContactId = ref("");
const joinGroupId = ref("");
const showCreateGroupModal = ref(false);
const showGroupInfo = ref(false);
const previewImage = ref(null);
const messageListRef = ref(null);

const connectionStatusText = computed(() => {
    switch (connectionStatus.value) {
        case "connected":
            return "已连接";
        case "connecting":
            return "连接中...";
        default:
            return "已断开";
    }
});

const currentChatName = computed(() => {
    if (!chatStore.currentChat) return "";
    if (chatStore.currentChat.type === "group") {
        return chatStore.currentGroup?.name || chatStore.currentChat.id;
    }
    const contact = chatStore.contacts.find(
        (c) => c.id === chatStore.currentChat.id
    );
    return contact?.nickname || chatStore.currentChat.id;
});

// Lifecycle
onMounted(async () => {
    connect();
    await chatStore.loadUserGroups();
});

onUnmounted(() => {
    disconnect();
});

// Watch for new messages to scroll
watch(
    () => chatStore.currentMessages.length,
    () => {
        nextTick(() => {
            messageListRef.value?.scrollToBottom();
        });
    }
);

// Methods
function handleLogout() {
    disconnect();
    chatStore.clearAll();
    authStore.logout();
    router.push("/login");
}

async function startUserChat(userId) {
    if (!userId) return;
    chatStore.addContact(userId);
    chatStore.setCurrentChat("user", userId);
    await chatStore.loadPrivateHistory(userId);
    newContactId.value = "";
    showGroupInfo.value = false;
}

async function startGroupChat(groupId) {
    if (!groupId) return;
    chatStore.setCurrentChat("group", groupId);
    await Promise.all([
        chatStore.loadGroupHistory(groupId),
        chatStore.loadGroupMembers(groupId),
    ]);
    showGroupInfo.value = false;
}

async function handleJoinGroup() {
    if (!joinGroupId.value) return;
    try {
        await chatStore.joinGroup(joinGroupId.value);
        toast.success("加入群组成功");
        joinGroupId.value = "";
    } catch (error) {
        toast.error(error.message || "加入群组失败");
    }
}

async function handleCreateGroup(data) {
    try {
        await chatStore.createGroup(data.name, data.description, data.memberIds);
        toast.success("群组创建成功");
        showCreateGroupModal.value = false;
    } catch (error) {
        toast.error(error.message || "创建群组失败");
    }
}

async function handleLeaveGroup() {
    if (!chatStore.currentChat?.id) return;
    if (!confirm("确定要退出该群组吗？")) return;
    try {
        await chatStore.leaveGroup(chatStore.currentChat.id);
        toast.success("已退出群组");
        showGroupInfo.value = false;
    } catch (error) {
        toast.error(error.message || "退出群组失败");
    }
}

function handleSendMessage(content) {
    if (!content || !chatStore.currentChat) return;

    if (connectionStatus.value !== "connected") {
        toast.error("连接已断开，请稍候重试");
        return;
    }

    try {
        if (chatStore.currentChat.type === "user") {
            sendPrivateMessage(chatStore.currentChat.id, content);
        } else {
            sendGroupMessage(chatStore.currentChat.id, content);
        }
    } catch (error) {
        toast.error(error.message || "发送失败");
    }
}

function handleFileSelect(file) {
    try {
        fileUpload.selectFile(file);
    } catch (error) {
        toast.error(error.message);
    }
}

async function handleSendFile() {
    if (!chatStore.currentChat) return;

    try {
        const result = await fileUpload.uploadFile();
        sendFileMessage(
            chatStore.currentChat.type,
            chatStore.currentChat.id,
            result
        );
        toast.success("文件发送成功");
    } catch (error) {
        toast.error(error.message || "文件上传失败");
    }
}

function showImagePreview(url) {
    previewImage.value = url;
}

function copyGroupId(groupId) {
    navigator.clipboard.writeText(groupId).then(
        () => toast.success("群ID已复制"),
        () => toast.error("复制失败")
    );
}
</script>
