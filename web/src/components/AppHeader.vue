<template>
  <header class="app-header">
    <div class="logo">
      <span class="icon">⚙️</span>
      <h1>Jenkins Flow</h1>
    </div>
    
    <div class="actions">
      <button class="settings-btn" @click="$emit('open-settings')" title="Settings">
        <span class="icon">⚙️</span>
      </button>
    </div>
  </header>
</template>

<script setup>
defineProps({
  isRunning: {
    type: Boolean,
    default: false
  }
})

defineEmits(['open-settings'])
</script>

<style scoped>
.app-header {
  height: 64px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: var(--bg-secondary);
}

.logo {
  display: flex;
  align-items: center;
  gap: 16px;
}

.logo h1 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.logo .icon {
  font-size: 24px;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  padding: 4px 10px;
  background: var(--bg-primary);
  border-radius: 20px;
  border: 1px solid var(--border-color);
}

.status-indicator.running {
  color: var(--status-running);
  border-color: var(--status-running);
  background: var(--status-running-bg);
}

.status-indicator .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: currentColor;
}

.settings-btn {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  padding: 8px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.settings-btn:hover {
  background: var(--bg-primary);
  color: var(--text-primary);
}

.settings-btn .icon {
  font-size: 20px;
}
</style>
