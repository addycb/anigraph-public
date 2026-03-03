<template>
  <v-card class="staff-preview-card" elevation="8">
    <!-- Top section: Avatar, Name, Primary Occupations -->
    <div class="staff-preview-card__header">
      <v-avatar size="64" class="staff-preview-card__avatar">
        <v-img v-if="staff.image" :src="staff.image" />
        <v-icon v-else size="32">mdi-account</v-icon>
      </v-avatar>
      <div class="staff-preview-card__info">
        <div class="staff-preview-card__name">{{ staff.label || staff.name || 'Unknown' }}</div>
        <div v-if="primaryOccupations.length > 0" class="staff-preview-card__occupations">
          {{ primaryOccupations.filter(o => o).join(', ') }}
        </div>
      </div>
    </div>

    <!-- Bottom section: Category & Roles in this anime -->
    <div v-if="categoryLabel || roles.length > 0" class="staff-preview-card__details">
      <div v-if="categoryLabel" class="staff-preview-card__category" :style="{ borderLeftColor: categoryColor }">
        {{ categoryLabel }}
      </div>
      <div v-if="roles.length > 0" class="staff-preview-card__roles">
        <v-chip
          v-for="role in roles"
          :key="role"
          size="x-small"
          variant="tonal"
          class="staff-preview-card__role-chip"
        >
          {{ role }}
        </v-chip>
      </div>
    </div>
  </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { STAFF_CATEGORIES } from '@/utils/staffCategories'

interface StaffPreviewProps {
  staff: {
    id?: string | number
    label?: string
    name?: string
    image?: string
    category?: string
  }
  roles?: string[]
  categoryColor?: string
  primaryOccupations?: string[]
}

const props = withDefaults(defineProps<StaffPreviewProps>(), {
  roles: () => [],
  categoryColor: '#9e9e9e',
  primaryOccupations: () => []
})

const categoryLabel = computed(() => {
  if (!props.staff.category || props.staff.category === 'other') return null
  const cat = STAFF_CATEGORIES.find(c => c.key === props.staff.category)
  return cat ? cat.title_en : null
})
</script>

<style scoped>
.staff-preview-card {
  width: 220px;
  max-width: 260px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: var(--shadow-glow);
  padding: 12px;
}

.staff-preview-card__header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.staff-preview-card__avatar {
  flex-shrink: 0;
}

.staff-preview-card__info {
  min-width: 0;
}

.staff-preview-card__name {
  font-size: 0.9rem;
  font-weight: 600;
  line-height: 1.3;
  word-wrap: break-word;
}

.staff-preview-card__occupations {
  font-size: 0.72rem;
  color: rgba(var(--v-theme-on-surface), 0.55);
  margin-top: 3px;
  line-height: 1.3;
}

.staff-preview-card__details {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid rgba(128, 128, 128, 0.2);
}

.staff-preview-card__category {
  font-size: 0.75rem;
  font-weight: 600;
  padding-left: 8px;
  border-left: 3px solid;
  margin-bottom: 6px;
}

.staff-preview-card__roles {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.staff-preview-card__role-chip {
  font-size: 0.7rem;
}
</style>
