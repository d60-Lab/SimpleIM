import { defineStore } from "pinia";
import { ref, computed } from "vue";

export const useAuthStore = defineStore("auth", () => {
  // State
  const token = ref(localStorage.getItem("token") || null);
  const user = ref(JSON.parse(localStorage.getItem("currentUser") || "null"));

  // Getters
  const isAuthenticated = computed(() => !!token.value && !!user.value);
  const currentUser = computed(() => user.value);
  const userId = computed(() => user.value?.user_id || null);
  const username = computed(() => user.value?.username || null);
  const nickname = computed(() => user.value?.nickname || null);

  // Actions
  async function login(usernameInput, password) {
    const response = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: usernameInput, password }),
    });

    const data = await response.json();

    if (!response.ok) {
      throw new Error(data.error || "登录失败");
    }

    token.value = data.token;
    user.value = {
      user_id: data.user_id,
      username: data.username,
      nickname: data.nickname,
    };

    localStorage.setItem("token", data.token);
    localStorage.setItem("currentUser", JSON.stringify(user.value));

    return data;
  }

  async function register(usernameInput, nicknameInput, password) {
    const response = await fetch("/api/auth/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        username: usernameInput,
        nickname: nicknameInput,
        password,
      }),
    });

    const data = await response.json();

    if (!response.ok) {
      throw new Error(data.error || "注册失败");
    }

    return data;
  }

  function logout() {
    token.value = null;
    user.value = null;
    localStorage.removeItem("token");
    localStorage.removeItem("currentUser");
  }

  function getAuthHeaders() {
    return {
      Authorization: `Bearer ${token.value}`,
    };
  }

  return {
    // State
    token,
    user,
    // Getters
    isAuthenticated,
    currentUser,
    userId,
    username,
    nickname,
    // Actions
    login,
    register,
    logout,
    getAuthHeaders,
  };
});
