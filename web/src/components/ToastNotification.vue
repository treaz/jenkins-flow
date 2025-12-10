<template>
  <TransitionGroup name="toast" tag="div" class="toast-container">
    <div
      v-for="toast in toasts"
      :key="toast.id"
      class="toast"
      :class="toast.type"
      @click="remove(toast.id)"
    >
      <div class="toast-icon">
        <span v-if="toast.type === 'success'">✓</span>
        <span v-else-if="toast.type === 'error'">✕</span>
        <span v-else>ℹ</span>
      </div>
      <div class="toast-content">
        <div class="toast-title" v-if="toast.title">{{ toast.title }}</div>
        <div class="toast-message">{{ toast.message }}</div>
      </div>
      <div class="toast-close">×</div>
    </div>
  </TransitionGroup>
</template>

<script setup>
import { ref } from 'vue'

const toasts = ref([])
let idCounter = 0

const add = ({ message, type = 'info', title = '', duration = 5000 }) => {
  const id = idCounter++
  toasts.value.push({ id, message, type, title })
  if (duration > 0) {
    setTimeout(() => remove(id), duration)
  }
}

const remove = (id) => {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index !== -1) {
    toasts.value.splice(index, 1)
  }
}

defineExpose({ add, remove })
</script>

<style scoped>
.toast-container {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 12px;
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  min-width: 300px;
  max-width: 450px;
  padding: 16px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: flex-start;
  gap: 12px;
  cursor: pointer;
  backdrop-filter: blur(8px);
  position: relative;
  overflow: hidden;
}

.toast::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
}

.toast.success::before {
  background-color: var(--status-success);
}

.toast.error::before {
  background-color: var(--status-failed);
}

.toast.info::before {
  background-color: var(--accent);
}

.toast-icon {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 14px;
  font-weight: bold;
  flex-shrink: 0;
}

.toast.success .toast-icon {
  color: var(--status-success);
  background: var(--status-success-bg);
}

.toast.error .toast-icon {
  color: var(--status-failed);
  background: var(--status-failed-bg);
}

.toast.info .toast-icon {
  color: var(--accent);
  background: rgba(88, 166, 255, 0.15);
}

.toast-content {
  flex: 1;
}

.toast-title {
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--text-primary);
  font-size: 14px;
}

.toast-message {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.toast-close {
  color: var(--text-muted);
  font-size: 20px;
  line-height: 1;
  opacity: 0.5;
  transition: opacity 0.2s;
}

.toast:hover .toast-close {
  opacity: 1;
}

/* Animations */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(30px) scale(0.95);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(30px) scale(0.95);
}
</style>
