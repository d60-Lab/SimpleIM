<template>
    <div
        class="min-h-screen bg-gradient-to-br from-blue-500 to-blue-600 flex items-center justify-center p-4"
    >
        <div class="bg-white rounded-2xl shadow-2xl p-8 w-full max-w-md">
            <h1 class="text-3xl font-bold text-center text-gray-800 mb-8">
                💬 聊天
            </h1>

            <!-- Login Form -->
            <div v-if="isLoginMode">
                <form @submit.prevent="handleLogin" class="space-y-6">
                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            用户名
                        </label>
                        <input
                            v-model="loginForm.username"
                            type="text"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请输入用户名"
                        />
                    </div>

                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            密码
                        </label>
                        <input
                            v-model="loginForm.password"
                            type="password"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请输入密码"
                        />
                    </div>

                    <button
                        type="submit"
                        :disabled="isLoading"
                        class="w-full py-3 bg-blue-500 hover:bg-blue-600 text-white font-semibold rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {{ isLoading ? "登录中..." : "登录" }}
                    </button>

                    <button
                        type="button"
                        @click="isLoginMode = false"
                        class="w-full py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-semibold rounded-lg transition"
                    >
                        没有账号？注册
                    </button>
                </form>
            </div>

            <!-- Register Form -->
            <div v-else>
                <form @submit.prevent="handleRegister" class="space-y-6">
                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            用户名
                        </label>
                        <input
                            v-model="registerForm.username"
                            type="text"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请输入用户名"
                        />
                    </div>

                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            昵称
                        </label>
                        <input
                            v-model="registerForm.nickname"
                            type="text"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请输入昵称"
                        />
                    </div>

                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            密码
                        </label>
                        <input
                            v-model="registerForm.password"
                            type="password"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请输入密码"
                        />
                    </div>

                    <div>
                        <label
                            class="block text-sm font-medium text-gray-700 mb-2"
                        >
                            确认密码
                        </label>
                        <input
                            v-model="registerForm.confirmPassword"
                            type="password"
                            required
                            class="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                            placeholder="请再次输入密码"
                        />
                        <p
                            v-if="passwordMismatch"
                            class="mt-1 text-sm text-red-500"
                        >
                            两次输入的密码不一致
                        </p>
                    </div>

                    <button
                        type="submit"
                        :disabled="isLoading || passwordMismatch"
                        class="w-full py-3 bg-green-500 hover:bg-green-600 text-white font-semibold rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {{ isLoading ? "注册中..." : "注册" }}
                    </button>

                    <button
                        type="button"
                        @click="isLoginMode = true"
                        class="w-full py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-semibold rounded-lg transition"
                    >
                        已有账号？登录
                    </button>
                </form>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, computed } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useToast } from "@/composables/useToast";

const router = useRouter();
const authStore = useAuthStore();
const toast = useToast();

const isLoginMode = ref(true);
const isLoading = ref(false);

const loginForm = ref({
    username: "",
    password: "",
});

const registerForm = ref({
    username: "",
    nickname: "",
    password: "",
    confirmPassword: "",
});

const passwordMismatch = computed(() => {
    return (
        registerForm.value.confirmPassword &&
        registerForm.value.password !== registerForm.value.confirmPassword
    );
});

async function handleLogin() {
    if (!loginForm.value.username || !loginForm.value.password) {
        toast.error("请填写用户名和密码");
        return;
    }

    isLoading.value = true;
    try {
        await authStore.login(loginForm.value.username, loginForm.value.password);
        toast.success("登录成功");
        router.push("/chat");
    } catch (error) {
        toast.error(error.message || "登录失败");
    } finally {
        isLoading.value = false;
    }
}

async function handleRegister() {
    if (
        !registerForm.value.username ||
        !registerForm.value.nickname ||
        !registerForm.value.password
    ) {
        toast.error("请填写所有字段");
        return;
    }

    if (registerForm.value.password !== registerForm.value.confirmPassword) {
        toast.error("两次输入的密码不一致");
        return;
    }

    if (registerForm.value.password.length < 6) {
        toast.error("密码长度至少6位");
        return;
    }

    isLoading.value = true;
    try {
        await authStore.register(
            registerForm.value.username,
            registerForm.value.nickname,
            registerForm.value.password
        );
        toast.success("注册成功，请登录");
        isLoginMode.value = true;
        registerForm.value = {
            username: "",
            nickname: "",
            password: "",
            confirmPassword: "",
        };
    } catch (error) {
        toast.error(error.message || "注册失败");
    } finally {
        isLoading.value = false;
    }
}
</script>
