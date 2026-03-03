<template>
  <div v-if="spacer" class="glass-pane"></div>
  <div v-else class="year-divider">
    <span class="year-label">{{ year || 'Unknown Year' }}</span>
    <span v-if="continued" class="year-continued">(continued)</span>
    <span class="year-count">{{ count }} {{ count === 1 ? countLabel : countLabel + 's' }}</span>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  spacer?: boolean
  year?: number | string
  count?: number
  countLabel?: string
  continued?: boolean
}>()
</script>

<style scoped>
.glass-pane {
  width: 100%;
  height: 100%;
  background: linear-gradient(135deg, rgba(var(--color-primary-rgb), 0.05) 0%, rgba(var(--color-accent-rgb), 0.08) 100%);
  backdrop-filter: blur(12px);
  border: 1px solid var(--color-primary-muted);
  border-radius: 8px;
  position: relative;
  overflow: hidden;
  transition: all 0.3s ease;
}

.glass-pane::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(
    circle at center,
    rgba(var(--color-text-rgb), 0.02) 0%,
    transparent 70%
  );
  opacity: 0;
  transition: opacity 0.3s ease;
}

.glass-pane::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(
    circle at 30% 30%,
    rgba(var(--color-primary-rgb), 0.08) 0%,
    transparent 60%
  );
  opacity: 0.6;
}

.glass-pane:hover::before {
  opacity: 1;
}

.year-divider {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 6px;
  padding: 16px 8px;
  border: 2px solid var(--color-primary-strong);
  background: linear-gradient(135deg, var(--color-primary-faint) 0%, rgba(var(--color-accent-rgb), 0.1) 100%);
  border-radius: 8px;
  animation: fadeIn 0.6s ease-out;
  width: 100%;
  height: 100%;
  transition: all 0.3s ease;
}

.year-divider:hover {
  border-color: var(--color-primary-border-focus);
  background: linear-gradient(135deg, var(--color-primary-muted) 0%, rgba(var(--color-accent-rgb), 0.15) 100%);
  transform: translateY(-2px);
}

.year-label {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-primary);
  white-space: nowrap;
  line-height: 1;
  text-align: center;
}

.year-continued {
  font-size: 0.75rem;
  font-weight: 500;
  color: rgba(var(--color-primary-rgb), 0.7);
  font-style: italic;
  line-height: 1;
}

.year-count {
  font-size: 0.7rem;
  font-weight: 500;
  color: rgba(var(--color-text-rgb), 0.5);
  white-space: nowrap;
  line-height: 1;
  text-align: center;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
