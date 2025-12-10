<template>
  <div class="workflow-view" v-if="workflow">
    <div class="workflow-header">
      <div class="workflow-info">
        <h2 class="workflow-name">{{ workflow.name }}</h2>
        <div class="workflow-meta">
          <StatusBadge :status="workflow.status" />
          <span v-if="workflow.startedAt" class="started-time">
            Started {{ formatTime(workflow.startedAt) }}
          </span>
          <span v-if="totalDuration" class="total-duration">
            {{ totalDuration }}
          </span>
        </div>
      </div>
    </div>
    
    <div v-if="workflow.error" class="workflow-error">
      {{ workflow.error }}
    </div>
    
    <div class="workflow-items">
      <div v-for="(item, index) in workflow.items" :key="index" class="workflow-item">
        <div class="item-connector" v-if="index > 0">
          <div class="connector-line"></div>
        </div>
        
        <StepCard
          v-if="item.isParallel"
          :name="item.parallel?.name || `Parallel Group ${index + 1}`"
          :status="item.parallel?.status || 'pending'"
          :is-parallel="true"
          :steps="item.parallel?.steps"
        />
        <StepCard
          v-else
          :name="item.step?.name || 'Unknown'"
          :instance="item.step?.instance"
          :job="item.step?.job"
          :status="item.step?.status || 'pending'"
          :build-url="item.step?.buildUrl"
          :error="item.step?.error"
          :started-at="item.step?.startedAt"
          :ended-at="item.step?.endedAt"
        />
      </div>
    </div>
  </div>
  
  <div class="empty-state" v-else>
    <div class="empty-icon">⚙️</div>
    <h3>No Workflow Running</h3>
    <p>Select a workflow and click Run to start execution.</p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import StepCard from './StepCard.vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  workflow: Object
})

const formatTime = (isoString) => {
  if (!isoString) return ''
  const date = new Date(isoString)
  return date.toLocaleTimeString()
}

const totalDuration = computed(() => {
  if (!props.workflow?.startedAt) return null
  
  const start = new Date(props.workflow.startedAt)
  const end = props.workflow.endedAt ? new Date(props.workflow.endedAt) : new Date()
  const diff = Math.floor((end - start) / 1000)
  
  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`
  return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`
})
</script>

<style scoped>
.workflow-view {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.workflow-header {
  margin-bottom: 24px;
}

.workflow-name {
  font-size: 20px;
  font-weight: 700;
  margin-bottom: 8px;
}

.workflow-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 13px;
  color: var(--text-secondary);
}

.workflow-error {
  margin-bottom: 24px;
  padding: 16px;
  background: var(--status-failed-bg);
  border-radius: var(--radius-md);
  color: var(--status-failed);
  font-family: monospace;
  font-size: 13px;
}

.workflow-items {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.workflow-item {
  position: relative;
}

.item-connector {
  display: flex;
  justify-content: center;
  padding: 8px 0;
}

.connector-line {
  width: 2px;
  height: 24px;
  background: var(--border-color);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 24px;
  text-align: center;
  background: var(--bg-secondary);
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-lg);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-state h3 {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.empty-state p {
  color: var(--text-secondary);
  font-size: 14px;
}
</style>
