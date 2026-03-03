<template>
  <div class="list-button-wrapper">
    <!-- Unauthenticated: plain button that prompts login -->
    <v-btn
      v-if="!isAuthenticated"
      icon
      :size="size"
      variant="flat"
      class="list-button"
      :class="{ 'bubble-mode': bubbleMode }"
      @click="requireLogin"
    >
      <v-icon :size="iconSize" class="list-button-icon">mdi-bookmark-outline</v-icon>
    </v-btn>

    <!-- Authenticated: full list menu -->
    <v-menu v-else offset-y :close-on-content-click="false">
      <template v-slot:activator="{ props: menuProps }">
        <v-btn
          v-bind="menuProps"
          icon
          :size="size"
          variant="flat"
          class="list-button"
          :class="{ 'in-lists': isInAnyList, 'bubble-mode': bubbleMode }"
        >
          <v-icon :size="iconSize" class="list-button-icon">
            {{ isInAnyList ? 'mdi-bookmark' : 'mdi-bookmark-outline' }}
          </v-icon>
        </v-btn>
      </template>

      <v-card min-width="300" max-width="400">
        <v-card-title class="d-flex align-center justify-space-between">
          <span>Add to List</span>
          <v-btn
            icon
            size="small"
            variant="text"
            @click="showCreateDialog = true"
          >
            <v-icon>mdi-plus</v-icon>
          </v-btn>
        </v-card-title>

        <v-divider></v-divider>

        <v-card-text class="pa-0">
          <v-list v-if="userLists.length > 0" density="compact">
            <v-list-item
              v-for="list in userLists"
              :key="list.id"
              @click="toggleList(list.id)"
            >
              <template v-slot:prepend>
                <v-checkbox
                  :model-value="isInList(list.id)"
                  hide-details
                  density="compact"
                  @click.stop="toggleList(list.id)"
                ></v-checkbox>
              </template>
              <v-list-item-title>{{ list.name }}</v-list-item-title>
              <v-list-item-subtitle v-if="list.itemCount !== undefined">
                {{ list.itemCount }} {{ list.itemCount === 1 ? 'item' : 'items' }}
              </v-list-item-subtitle>
            </v-list-item>
          </v-list>

          <div v-else class="pa-4 text-center text-medium-emphasis">
            <p class="text-body-2">No lists yet</p>
            <v-btn
              color="primary"
              variant="text"
              size="small"
              @click="showCreateDialog = true"
            >
              Create your first list
            </v-btn>
          </div>
        </v-card-text>
      </v-card>
    </v-menu>

    <!-- Create List Dialog -->
    <v-dialog v-model="showCreateDialog" max-width="500">
      <v-card>
        <v-card-title>Create New List</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="newListName"
            label="List Name"
            variant="outlined"
            density="comfortable"
            autofocus
            @keyup.enter="createNewList"
          ></v-text-field>
          <v-textarea
            v-model="newListDescription"
            label="Description (optional)"
            variant="outlined"
            density="comfortable"
            rows="3"
          ></v-textarea>
          <v-checkbox
            v-model="newListIsPublic"
            label="Make this list public"
            hide-details
          ></v-checkbox>

          <!-- Error Alert -->
          <v-alert
            v-if="showError"
            type="error"
            density="compact"
            closable
            @click:close="showError = false"
            class="mt-3"
          >
            {{ errorMessage }}
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="showCreateDialog = false">Cancel</v-btn>
          <v-btn
            color="primary"
            variant="flat"
            @click="createNewList"
            :loading="creatingList"
            :disabled="!newListName.trim()"
          >
            Create
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuth } from '~/composables/useAuth'
import { useLoginRequired } from '~/composables/useLoginRequired'
import { useLists } from '~/composables/useLists'

interface Props {
  animeId: number | string
  bubbleMode?: boolean
  size?: 'x-small' | 'small' | 'default' | 'large' | 'x-large'
}

const props = withDefaults(defineProps<Props>(), {
  bubbleMode: false,
  size: 'large'
})

// Map button size to icon size
const iconSize = computed(() => {
  switch (props.size) {
    case 'x-small': return 16
    case 'small': return 20
    case 'default': return 24
    case 'large': return 28
    case 'x-large': return 32
    default: return 28
  }
})

const { isAuthenticated } = useAuth()
const { requireLogin } = useLoginRequired()
const { userLists, fetchLists, createList, toggleInList, isInList: checkIsInList } = useLists()

const showCreateDialog = ref(false)
const newListName = ref('')
const newListDescription = ref('')
const newListIsPublic = ref(false)
const creatingList = ref(false)
const errorMessage = ref('')
const showError = ref(false)

const isInAnyList = computed(() => {
  return userLists.value.some(list => checkIsInList(list.id, props.animeId))
})

const isInList = (listId: number) => {
  return checkIsInList(listId, props.animeId)
}

const toggleList = async (listId: number) => {
  await toggleInList(listId, props.animeId)
}

const createNewList = async () => {
  if (!newListName.value.trim()) return

  creatingList.value = true
  errorMessage.value = ''
  showError.value = false

  try {
    const list = await createList(
      newListName.value.trim(),
      newListDescription.value.trim() || undefined,
      newListIsPublic.value
    )

    // Reset form
    newListName.value = ''
    newListDescription.value = ''
    newListIsPublic.value = false
    showCreateDialog.value = false

    // Automatically add anime to the new list
    await toggleInList(list.id, props.animeId)
  } catch (error: any) {
    errorMessage.value = error.data?.message || error.message || 'Failed to create list'
    showError.value = true
  } finally {
    creatingList.value = false
  }
}

// Fetch lists on mount if not already loaded
onMounted(() => {
  fetchLists()
})
</script>

<style scoped>
.list-button {
  background-color: rgba(var(--color-overlay-rgb), 0.75) !important;
  backdrop-filter: blur(8px);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
  border: 2px solid rgba(var(--color-text-rgb), 0.1) !important;
  transition: all 0.2s ease;
}

.list-button:hover {
  transform: scale(1.1);
  background-color: rgba(var(--color-overlay-rgb), 0.85) !important;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
}

.list-button.in-lists {
  border-color: var(--color-primary-border-focus) !important;
}

.list-button-icon {
  color: var(--color-text) !important;
}

.list-button.in-lists :deep(.v-icon) {
  color: var(--color-primary) !important;
}

/* Bubble mode - when inside action bubble */
.list-button.bubble-mode {
  background-color: transparent !important;
  backdrop-filter: none !important;
  box-shadow: none !important;
  border: none !important;
}

.list-button.bubble-mode:hover {
  transform: none;
  background-color: rgba(var(--color-text-rgb), 0.1) !important;
}
</style>
