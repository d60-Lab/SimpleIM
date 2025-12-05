import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { useAuthStore } from "./auth";

export const useChatStore = defineStore("chat", () => {
  // State
  const messages = ref({}); // { 'user:id': [], 'group:id': [] }
  const contacts = ref([]);
  const groups = ref([]);
  const groupMembers = ref({}); // { groupId: [members] }
  const currentChat = ref(null); // { type: 'user' | 'group', id: string }
  const unreadCounts = ref({}); // { 'user:id': count, 'group:id': count }

  // Getters
  const currentChatKey = computed(() => {
    if (!currentChat.value) return null;
    return `${currentChat.value.type}:${currentChat.value.id}`;
  });

  const currentMessages = computed(() => {
    if (!currentChatKey.value) return [];
    return messages.value[currentChatKey.value] || [];
  });

  const currentGroup = computed(() => {
    if (!currentChat.value || currentChat.value.type !== "group") return null;
    return groups.value.find(
      (g) => (g.group_id || g.id) === currentChat.value.id
    );
  });

  const currentGroupMembers = computed(() => {
    if (!currentChat.value || currentChat.value.type !== "group") return [];
    return groupMembers.value[currentChat.value.id] || [];
  });

  // Actions
  function setCurrentChat(type, id) {
    currentChat.value = { type, id };
    clearUnreadCount(type, id);
  }

  function clearCurrentChat() {
    currentChat.value = null;
  }

  function addMessage(chatKey, message) {
    if (!messages.value[chatKey]) {
      messages.value[chatKey] = [];
    }
    messages.value[chatKey].push(message);
  }

  function setMessages(chatKey, messageList) {
    messages.value[chatKey] = messageList;
  }

  function addContact(userId) {
    if (!contacts.value.find((c) => c.id === userId)) {
      contacts.value.push({
        id: userId,
        nickname: userId,
      });
    }
  }

  function setContacts(contactList) {
    contacts.value = contactList;
  }

  function setGroups(groupList) {
    groups.value = groupList;
  }

  function addGroup(group) {
    const existingIndex = groups.value.findIndex(
      (g) => (g.group_id || g.id) === (group.group_id || group.id)
    );
    if (existingIndex >= 0) {
      groups.value[existingIndex] = group;
    } else {
      groups.value.push(group);
    }
  }

  function removeGroup(groupId) {
    groups.value = groups.value.filter(
      (g) => (g.group_id || g.id) !== groupId
    );
    delete messages.value[`group:${groupId}`];
    delete groupMembers.value[groupId];
    if (currentChat.value?.type === "group" && currentChat.value.id === groupId) {
      currentChat.value = null;
    }
  }

  function setGroupMembers(groupId, members) {
    groupMembers.value[groupId] = members;
  }

  function incrementUnreadCount(type, id) {
    const key = `${type}:${id}`;
    if (!unreadCounts.value[key]) {
      unreadCounts.value[key] = 0;
    }
    unreadCounts.value[key]++;
  }

  function clearUnreadCount(type, id) {
    const key = `${type}:${id}`;
    unreadCounts.value[key] = 0;
  }

  function getUnreadCount(type, id) {
    return unreadCounts.value[`${type}:${id}`] || 0;
  }

  function getLastMessage(chatKey) {
    const msgs = messages.value[chatKey];
    if (!msgs || msgs.length === 0) return null;
    return msgs[msgs.length - 1];
  }

  function getMessagePreview(chatKey) {
    const lastMsg = getLastMessage(chatKey);
    if (!lastMsg) return "暂无消息";

    const content = lastMsg.content;
    const contentType = lastMsg.contentType || "text";

    if (contentType === "image") return "[图片]";
    if (contentType === "file") return "[文件]";
    if (contentType === "voice") return "[语音]";
    if (contentType === "video") return "[视频]";

    if (typeof content === "string") {
      return content.length > 20 ? content.slice(0, 20) + "..." : content;
    }

    if (content?.text) {
      const text = content.text;
      return text.length > 20 ? text.slice(0, 20) + "..." : text;
    }

    return "暂无消息";
  }

  // API Actions
  async function loadUserGroups() {
    const authStore = useAuthStore();
    try {
      const response = await fetch("/api/groups/my", {
        headers: authStore.getAuthHeaders(),
      });
      const data = await response.json();
      if (response.ok && data.groups) {
        setGroups(data.groups);
      }
    } catch (error) {
      console.error("Failed to load groups:", error);
    }
  }

  async function loadPrivateHistory(userId) {
    const authStore = useAuthStore();
    try {
      const response = await fetch(`/api/messages/private/${userId}?limit=50`, {
        headers: authStore.getAuthHeaders(),
      });
      const data = await response.json();
      if (response.ok && data.messages) {
        const chatKey = `user:${userId}`;
        const historyMessages = data.messages.map((msg) => ({
          id: msg.message_id,
          from: msg.from_user_id,
          content: parseMessageContent(msg.content),
          contentType: getContentTypeFromMsg(msg),
          timestamp: new Date(msg.created_at).getTime(),
          isFromMe: msg.from_user_id === authStore.userId,
        }));
        setMessages(chatKey, historyMessages);
      }
    } catch (error) {
      console.error("Failed to load private history:", error);
    }
  }

  async function loadGroupHistory(groupId) {
    const authStore = useAuthStore();
    try {
      const response = await fetch(`/api/messages/group/${groupId}?limit=50`, {
        headers: authStore.getAuthHeaders(),
      });
      const data = await response.json();
      if (response.ok && data.messages) {
        const chatKey = `group:${groupId}`;
        const historyMessages = data.messages.map((msg) => ({
          id: msg.message_id,
          from: msg.from_user_id,
          content: parseMessageContent(msg.content),
          contentType: getContentTypeFromMsg(msg),
          timestamp: new Date(msg.created_at).getTime(),
          isFromMe: msg.from_user_id === authStore.userId,
        }));
        setMessages(chatKey, historyMessages);
      }
    } catch (error) {
      console.error("Failed to load group history:", error);
    }
  }

  async function loadGroupMembers(groupId) {
    const authStore = useAuthStore();
    try {
      const response = await fetch(`/api/groups/${groupId}/members`, {
        headers: authStore.getAuthHeaders(),
      });
      const data = await response.json();
      if (response.ok && data.members) {
        setGroupMembers(groupId, data.members);
      }
    } catch (error) {
      console.error("Failed to load group members:", error);
    }
  }

  async function createGroup(name, description, memberIds) {
    const authStore = useAuthStore();
    const response = await fetch("/api/groups", {
      method: "POST",
      headers: {
        ...authStore.getAuthHeaders(),
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        name,
        description,
        member_ids: memberIds,
      }),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "创建群组失败");
    }
    await loadUserGroups();
    return data;
  }

  async function joinGroup(groupId) {
    const authStore = useAuthStore();
    const response = await fetch(`/api/groups/${groupId}/join`, {
      method: "POST",
      headers: {
        ...authStore.getAuthHeaders(),
        "Content-Type": "application/json",
      },
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "加入群组失败");
    }
    await loadUserGroups();
    return data;
  }

  async function leaveGroup(groupId) {
    const authStore = useAuthStore();
    const response = await fetch(`/api/groups/${groupId}/leave`, {
      method: "POST",
      headers: authStore.getAuthHeaders(),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "退出群组失败");
    }
    removeGroup(groupId);
    return data;
  }

  // Helper functions
  function parseMessageContent(content) {
    if (typeof content === "string") {
      try {
        return JSON.parse(content);
      } catch {
        return content;
      }
    }
    return content;
  }

  function getContentTypeFromMsg(msg) {
    const content = parseMessageContent(msg.content);
    if (msg.msg_type === 4 || content?.file_type === "image") return "image";
    if (msg.msg_type === 7 || (content?.file_id && content?.file_type !== "image")) return "file";
    if (msg.msg_type === 5) return "voice";
    if (msg.msg_type === 6) return "video";
    return "text";
  }

  function clearAll() {
    messages.value = {};
    contacts.value = [];
    groups.value = [];
    groupMembers.value = {};
    currentChat.value = null;
    unreadCounts.value = {};
  }

  return {
    // State
    messages,
    contacts,
    groups,
    groupMembers,
    currentChat,
    unreadCounts,
    // Getters
    currentChatKey,
    currentMessages,
    currentGroup,
    currentGroupMembers,
    // Actions
    setCurrentChat,
    clearCurrentChat,
    addMessage,
    setMessages,
    addContact,
    setContacts,
    setGroups,
    addGroup,
    removeGroup,
    setGroupMembers,
    incrementUnreadCount,
    clearUnreadCount,
    getUnreadCount,
    getLastMessage,
    getMessagePreview,
    // API Actions
    loadUserGroups,
    loadPrivateHistory,
    loadGroupHistory,
    loadGroupMembers,
    createGroup,
    joinGroup,
    leaveGroup,
    // Helpers
    parseMessageContent,
    getContentTypeFromMsg,
    clearAll,
  };
});
