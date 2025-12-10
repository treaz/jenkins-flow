<template>
  <div class="step-card" :class="{ 'is-parallel': isParallel }">
    <div class="step-header">
      <div class="step-info">
        <h3 class="step-name">{{ name }}</h3>
        <div class="step-meta" v-if="!isParallel">
          <span class="instance" v-if="instance">{{ instance }}</span>
          <span class="job" v-if="job">{{ job }}</span>
        </div>
      </div>
      <component
        :is="statusLinkTag"
        v-bind="statusLinkProps"
        class="status-link"
      >
        <StatusBadge :status="status" />
      </component>
    </div>
    
    <div v-if="buildUrl" class="build-link">
      <a :href="buildUrl" target="_blank" rel="noopener">View Build →</a>
    </div>
    
    <div v-if="error" class="error-message">
      {{ error }}
    </div>
    
    <div v-if="duration" class="duration">
      {{ duration }}
    </div>
    
    <!-- Parallel steps container -->
    <div v-if="isParallel && steps" class="parallel-steps">
      <StepCard
        v-for="(step, index) in steps"
        :key="index"
        :name="step.name"
        :instance="step.instance"
        :job="step.job"
        :status="step.status"
        :build-url="step.buildUrl"
        :error="step.error"
        :started-at="step.startedAt"
        :ended-at="step.endedAt"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  name: { type: String, required: true },
  instance: String,
  job: String,
  status: { type: String, required: true },
  buildUrl: String,
  error: String,
  startedAt: String,
  endedAt: String,
  isParallel: Boolean,
  steps: Array
})

const duration = computed(() => {
  if (!props.startedAt) return null
  
  const start = new Date(props.startedAt)
  const end = props.endedAt ? new Date(props.endedAt) : new Date()
  const diff = Math.floor((end - start) / 1000)
  
  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`
  return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`
})

const hasBuildLink = computed(() => Boolean(props.buildUrl))

const statusLinkTag = computed(() => (hasBuildLink.value ? 'a' : 'div'))

const statusLinkProps = computed(() => {
  if (!hasBuildLink.value) return {}
  return {
    href: props.buildUrl,
    target: '_blank',
    rel: 'noopener',
    title: 'Open Jenkins build',
    'aria-label': 'Open Jenkins build (opens in new tab)'
  }
})
</script>

<style scoped>
.step-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 16px;
  transition: border-color 0.2s;
}

.step-card:hover {
  border-color: var(--accent);
}

.step-card.is-parallel {
  background: var(--bg-tertiary);
}

.step-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.status-link {
  text-decoration: none;
  display: inline-flex;
}

.status-link[href] {
  cursor: pointer;
}

.step-info {
  flex: 1;
  min-width: 0;
}

.step-name {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--text-primary);
}

.step-meta {
  display: flex;
  gap: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}

.step-meta .instance::before {
  content: '⬡ ';
  opacity: 0.6;
}

.step-meta .job::before {
  content: '📋 ';
  opacity: 0.6;
}

.build-link {
  margin-top: 12px;
}

.build-link a {
  color: var(--accent);
  text-decoration: none;
  font-size: 13px;
  font-weight: 500;
}

.build-link a:hover {
  text-decoration: underline;
}

.error-message {
  margin-top: 12px;
  padding: 10px 12px;
  background: var(--status-failed-bg);
  border-radius: var(--radius-sm);
  color: var(--status-failed);
  font-size: 13px;
  font-family: monospace;
}

.duration {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.parallel-steps {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
  margin-top: 16px;
}
</style>
