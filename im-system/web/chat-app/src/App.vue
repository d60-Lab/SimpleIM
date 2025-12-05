<template>
    <div id="app" class="min-h-screen bg-gray-100">
        <router-view />

        <!-- Toast Container -->
        <div class="fixed bottom-4 right-4 z-50 space-y-2">
            <transition-group name="toast">
                <div
                    v-for="toast in toasts"
                    :key="toast.id"
                    class="px-4 py-3 rounded-lg shadow-lg text-white animate-fade-in"
                    :class="{
                        'bg-green-500': toast.type === 'success',
                        'bg-red-500': toast.type === 'error',
                        'bg-yellow-500': toast.type === 'warning',
                        'bg-blue-500': toast.type === 'info',
                    }"
                >
                    {{ toast.message }}
                </div>
            </transition-group>
        </div>
    </div>
</template>

<script setup>
import { useToast } from "@/composables/useToast";

const { toasts } = useToast();
</script>

<style>
.toast-enter-active,
.toast-leave-active {
    transition: all 0.3s ease;
}

.toast-enter-from,
.toast-leave-to {
    opacity: 0;
    transform: translateX(100%);
}
</style>
