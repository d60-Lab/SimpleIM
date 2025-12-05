import { ref, onUnmounted } from "vue";
import { useAuthStore } from "@/stores/auth";
import { useChatStore } from "@/stores/chat";

export function useWebSocket() {
  const authStore = useAuthStore();
  const chatStore = useChatStore();

  const ws = ref(null);
  const connectionStatus = ref("disconnected"); // 'connected' | 'connecting' | 'disconnected'
  const reconnectAttempts = ref(0);
  let reconnectTimer = null;
  let heartbeatTimer = null;

  const WS_URL = `${window.location.protocol === "https:" ? "wss://" : "ws://"}${window.location.host}/ws`;

  function connect() {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) return;
    if (!authStore.token) return;

    connectionStatus.value = "connecting";
    ws.value = new WebSocket(`${WS_URL}?token=${authStore.token}`);

    ws.value.onopen = () => {
      console.log("WebSocket connected");
      connectionStatus.value = "connected";
      reconnectAttempts.value = 0;
      startHeartbeat();
    };

    ws.value.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleMessage(msg);
      } catch (error) {
        console.error("Failed to parse WebSocket message:", error);
      }
    };

    ws.value.onclose = (event) => {
      console.log("WebSocket disconnected", event.code, event.reason);
      connectionStatus.value = "disconnected";
      stopHeartbeat();
      scheduleReconnect();
    };

    ws.value.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
  }

  function disconnect() {
    stopHeartbeat();
    clearReconnectTimer();
    if (ws.value) {
      ws.value.close();
      ws.value = null;
    }
    connectionStatus.value = "disconnected";
  }

  function scheduleReconnect() {
    if (reconnectTimer) return;
    if (!authStore.token) return;

    reconnectAttempts.value++;
    const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.value), 30000);
    console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts.value})`);

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      if (authStore.token) {
        connect();
      }
    }, delay);
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function startHeartbeat() {
    stopHeartbeat();
    heartbeatTimer = setInterval(() => {
      if (ws.value && ws.value.readyState === WebSocket.OPEN) {
        ws.value.send(
          JSON.stringify({
            type: 99,
            content: { timestamp: Date.now() },
          })
        );
      }
    }, 30000);
  }

  function stopHeartbeat() {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
  }

  function handleMessage(msg) {
    console.log("Received message:", msg);

    switch (msg.type) {
      case 0: // System message
      case 1: // Private message
        handlePrivateMessage(msg);
        break;
      case 2: // Group message
        handleGroupMessage(msg);
        break;
      case 100: // Heartbeat response
        // Ignore heartbeat responses
        break;
      default:
        console.log("Unknown message type:", msg.type);
    }
  }

  function handlePrivateMessage(msg) {
    const fromUser = msg.from;
    const isFromMe = fromUser === authStore.userId;
    const chatId = isFromMe ? msg.to : fromUser;
    const chatKey = `user:${chatId}`;

    chatStore.addContact(chatId);

    const contentType = getContentType(msg.type, msg.content);
    const content = parseContent(contentType, msg.content);

    const messageData = {
      id: msg.message_id || Date.now().toString(),
      from: fromUser,
      content: content,
      contentType: contentType,
      timestamp: msg.timestamp || Date.now(),
      isFromMe,
    };

    chatStore.addMessage(chatKey, messageData);

    // If not in current chat, increment unread count
    if (!(chatStore.currentChat?.type === "user" && chatStore.currentChat.id === chatId)) {
      if (!isFromMe) {
        chatStore.incrementUnreadCount("user", chatId);
      }
    }
  }

  function handleGroupMessage(msg) {
    const groupId = msg.group_id || msg.to;
    const fromUser = msg.from;
    const isFromMe = fromUser === authStore.userId;
    const chatKey = `group:${groupId}`;

    const contentType = getContentType(msg.type, msg.content);
    const content = parseContent(contentType, msg.content);

    const messageData = {
      id: msg.message_id || Date.now().toString(),
      from: fromUser,
      content: content,
      contentType: contentType,
      timestamp: msg.timestamp || Date.now(),
      isFromMe,
    };

    chatStore.addMessage(chatKey, messageData);

    // If not in current chat, increment unread count
    if (!(chatStore.currentChat?.type === "group" && chatStore.currentChat.id === groupId)) {
      if (!isFromMe) {
        chatStore.incrementUnreadCount("group", groupId);
      }
    }
  }

  function getContentType(msgType, content) {
    if (msgType === 4 || (typeof content === "object" && content?.file_type === "image")) {
      return "image";
    }
    if (msgType === 7 || (typeof content === "object" && content?.file_id && content?.file_type !== "image")) {
      return "file";
    }
    if (msgType === 5) return "voice";
    if (msgType === 6) return "video";
    return "text";
  }

  function parseContent(contentType, content) {
    if (contentType === "text") {
      if (typeof content === "string") return content;
      if (content?.text) return content.text;
      return JSON.stringify(content);
    }
    return content;
  }

  function sendMessage(type, to, content, groupId = null) {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      throw new Error("WebSocket not connected");
    }

    const msg = {
      type,
      to,
      content,
      timestamp: Date.now(),
    };

    if (groupId) {
      msg.group_id = groupId;
    }

    ws.value.send(JSON.stringify(msg));
  }

  function sendPrivateMessage(to, text) {
    sendMessage(1, to, { text });

    // Add message to local store
    const chatKey = `user:${to}`;
    const messageData = {
      id: Date.now().toString(),
      from: authStore.userId,
      content: text,
      contentType: "text",
      timestamp: Date.now(),
      isFromMe: true,
    };
    chatStore.addMessage(chatKey, messageData);
  }

  function sendGroupMessage(groupId, text) {
    sendMessage(2, groupId, { text }, groupId);

    // Add message to local store
    const chatKey = `group:${groupId}`;
    const messageData = {
      id: Date.now().toString(),
      from: authStore.userId,
      content: text,
      contentType: "text",
      timestamp: Date.now(),
      isFromMe: true,
    };
    chatStore.addMessage(chatKey, messageData);
  }

  function sendFileMessage(chatType, to, fileData) {
    const isImage = fileData.file_type === "image";
    const msgType = isImage ? 4 : 7;

    const content = {
      file_id: fileData.file_id,
      file_name: fileData.file_name,
      file_size: fileData.file_size,
      file_type: fileData.file_type,
      mime_type: fileData.mime_type,
      url: fileData.url,
      thumbnail_url: fileData.thumbnail_url,
    };

    if (chatType === "user") {
      sendMessage(msgType, to, content);
    } else {
      sendMessage(msgType, to, content, to);
    }

    // Add message to local store
    const chatKey = `${chatType}:${to}`;
    const messageData = {
      id: Date.now().toString(),
      from: authStore.userId,
      content: content,
      contentType: isImage ? "image" : "file",
      timestamp: Date.now(),
      isFromMe: true,
    };
    chatStore.addMessage(chatKey, messageData);
  }

  // Cleanup on unmount
  onUnmounted(() => {
    disconnect();
  });

  return {
    ws,
    connectionStatus,
    connect,
    disconnect,
    sendPrivateMessage,
    sendGroupMessage,
    sendFileMessage,
  };
}
