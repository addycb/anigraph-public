<template>
  <div>
    <!-- Two-tier: Group panels containing child category panels -->
    <div
      v-if="staff && staff.length > 0"
      class="staff-view-content"
    >
      <v-expansion-panels class="mt-2" v-model="groupsOpen" multiple>
        <!-- Parent group panels -->
        <template v-for="group in categoryGroups" :key="group.key">
          <v-expansion-panel
            v-if="
              getGroupStaffCount(group.key) > 0 &&
              !(hideProducers && group.key === 'production_group')
            "
            :value="group.key"
          >
            <v-expansion-panel-title>
              <div class="d-flex align-center w-100">
                <v-checkbox
                  :model-value="isGroupStaffAllSelected(group.key)"
                  :indeterminate="
                    !isGroupStaffAllSelected(group.key) &&
                    isGroupStaffSomeSelected(group.key)
                  "
                  @click.stop="toggleGroupStaff(group.key)"
                  hide-details
                  density="compact"
                  class="mr-2"
                ></v-checkbox>
                <div
                  class="group-color-indicator mr-2"
                  :style="{ backgroundColor: groupColors[group.key] }"
                ></div>
                <span class="text-h6">{{ group.title_en }}</span>
                <span class="text-caption text-grey ml-2"
                  >({{ getGroupStaffCount(group.key) }} staff)</span
                >
              </div>
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <!-- Child category panels inside group -->
              <v-expansion-panels
                v-model="childCategoriesOpen[group.key]"
                multiple
                class="child-category-panels"
              >
                <template
                  v-for="childKey in group.children"
                  :key="childKey"
                >
                  <v-expansion-panel
                    v-if="
                      categorizedStaff[childKey] &&
                      categorizedStaff[childKey].length > 0
                    "
                    :value="childKey"
                  >
                    <v-expansion-panel-title class="child-panel-title">
                      <div class="d-flex align-center w-100">
                        <v-checkbox
                          :model-value="isCategoryAllSelected(childKey)"
                          :indeterminate="
                            !isCategoryAllSelected(childKey) &&
                            isCategorySomeSelected(childKey)
                          "
                          @click.stop="toggleCategoryStaff(childKey)"
                          hide-details
                          density="compact"
                          class="mr-2"
                        ></v-checkbox>
                        <span class="text-subtitle-1">{{
                          getCategoryTitleShort(childKey)
                        }}</span>
                        <span class="text-caption text-grey ml-2"
                          >({{ categorizedStaff[childKey].length }})</span
                        >
                      </div>
                    </v-expansion-panel-title>
                    <v-expansion-panel-text>
                      <v-list density="compact">
                        <v-list-item
                          v-for="(staffMember, idx) in categorizedStaff[childKey]"
                          :key="idx"
                          @click="
                            staffMember.staff?.staff_id &&
                            toggleStaff(staffMember.staff.staff_id)
                          "
                          class="cursor-pointer"
                        >
                          <template #prepend>
                            <v-checkbox
                              v-if="staffMember.staff?.staff_id"
                              :model-value="
                                selectedStaff.has(staffMember.staff.staff_id)
                              "
                              @click.stop="
                                toggleStaff(staffMember.staff.staff_id)
                              "
                              hide-details
                              density="compact"
                              class="mr-2"
                            ></v-checkbox>
                          </template>
                          <div class="staff-list-scrollable">
                            <v-avatar
                              v-if="staffMember.staff?.image"
                              size="small"
                              class="flex-shrink-0"
                            >
                              <v-img :src="staffMember.staff.image"></v-img>
                            </v-avatar>
                            <v-avatar
                              v-else
                              color="grey"
                              size="small"
                              class="flex-shrink-0"
                            >
                              <v-icon size="small">mdi-account</v-icon>
                            </v-avatar>
                            <div class="staff-list-text">
                              <v-list-item-title>{{
                                staffMember.staff?.name_en ||
                                staffMember.staff?.name_ja ||
                                "Unknown"
                              }}</v-list-item-title>
                              <v-list-item-subtitle>{{
                                Array.isArray(staffMember.role)
                                  ? staffMember.role.join(", ")
                                  : staffMember.role || "Unknown Role"
                              }}</v-list-item-subtitle>
                            </div>
                          </div>
                          <template #append>
                            <v-btn
                              v-if="staffMember.staff?.staff_id"
                              icon
                              size="small"
                              variant="text"
                              :to="`/staff/${staffMember.staff.staff_id}`"
                              @click.stop
                            >
                              <v-icon size="small">mdi-open-in-new</v-icon>
                            </v-btn>
                          </template>
                        </v-list-item>
                      </v-list>
                    </v-expansion-panel-text>
                  </v-expansion-panel>
                </template>
              </v-expansion-panels>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </template>

        <!-- Uncategorized Staff (Other group) -->
        <v-expansion-panel
          v-if="uncategorizedStaff.length > 0"
          value="other"
        >
          <v-expansion-panel-title>
            <div class="d-flex align-center w-100">
              <v-checkbox
                :model-value="isCategoryAllSelected('other')"
                :indeterminate="
                  !isCategoryAllSelected('other') &&
                  isCategorySomeSelected('other')
                "
                @click.stop="toggleCategoryStaff('other')"
                hide-details
                density="compact"
                class="mr-2"
              ></v-checkbox>
              <div
                class="group-color-indicator mr-2"
                :style="{ backgroundColor: groupColors.other }"
              ></div>
              <span class="text-h6">Other Staff</span>
              <span class="text-caption text-grey ml-2"
                >({{ uncategorizedStaff.length }})</span
              >
            </div>
          </v-expansion-panel-title>
          <v-expansion-panel-text>
            <v-list density="compact">
              <v-list-item
                v-for="(staffMember, idx) in uncategorizedStaff"
                :key="idx"
                @click="
                  staffMember.staff?.staff_id &&
                  toggleStaff(staffMember.staff.staff_id)
                "
                class="cursor-pointer"
              >
                <template #prepend>
                  <v-checkbox
                    v-if="staffMember.staff?.staff_id"
                    :model-value="
                      selectedStaff.has(staffMember.staff.staff_id)
                    "
                    @click.stop="
                      toggleStaff(staffMember.staff.staff_id)
                    "
                    hide-details
                    density="compact"
                    class="mr-2"
                  ></v-checkbox>
                </template>
                <div class="staff-list-scrollable">
                  <v-avatar
                    v-if="staffMember.staff?.image"
                    size="small"
                    class="flex-shrink-0"
                  >
                    <v-img :src="staffMember.staff.image"></v-img>
                  </v-avatar>
                  <v-avatar
                    v-else
                    color="grey"
                    size="small"
                    class="flex-shrink-0"
                  >
                    <v-icon size="small">mdi-account</v-icon>
                  </v-avatar>
                  <div class="staff-list-text">
                    <v-list-item-title>{{
                      staffMember.staff?.name_en ||
                      staffMember.staff?.name_ja ||
                      "Unknown"
                    }}</v-list-item-title>
                    <v-list-item-subtitle>{{
                      Array.isArray(staffMember.role)
                        ? staffMember.role.join(", ")
                        : staffMember.role || "Unknown Role"
                    }}</v-list-item-subtitle>
                  </div>
                </div>
                <template #append>
                  <v-btn
                    v-if="staffMember.staff?.staff_id"
                    icon
                    size="small"
                    variant="text"
                    :to="`/staff/${staffMember.staff.staff_id}`"
                    @click.stop
                  >
                    <v-icon size="small">mdi-open-in-new</v-icon>
                  </v-btn>
                </template>
              </v-list-item>
            </v-list>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </div>

    <!-- Empty State for Staff View -->
    <div
      v-if="!staff || staff.length === 0"
      class="text-center py-8"
    >
      <v-icon size="48" color="grey-lighten-1">mdi-account-off</v-icon>
      <p class="text-body-1 mt-3 text-grey">
        No staff information available.
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import {
  STAFF_CATEGORIES,
  CATEGORY_GROUPS,
  CATEGORY_TO_GROUP,
  getCategoryByKey,
  getGroupByKey,
  type CategoryGroup,
} from "@/utils/staffCategories";

const props = defineProps<{
  staff: any[] | undefined;
  categoryGroups: CategoryGroup[];
  categorizedStaff: Record<string, any[]>;
  uncategorizedStaff: any[];
  groupColors: Record<string, string>;
  hideProducers: boolean;
  graphViewMode: "graph" | "staff";
  categoryStaffIdsMap: Record<string, string[]>;
  groupStaffIdsMap: Record<string, string[]>;
  focusCategory?: string | null;
}>();

const emit = defineEmits<{
  staffChanged: [];
}>();

const selectedStaff = defineModel<Set<string>>("selectedStaff", {
  required: true,
});

const groupsOpen = ref<string[]>([]);
const childCategoriesOpen = ref<Record<string, string[]>>({});

// Initialize childCategoriesOpen for each group — start collapsed (populated by initializeGroupsOpen)
CATEGORY_GROUPS.forEach((group) => {
  childCategoriesOpen.value[group.key] = [];
});

const getGroupStaffCount = (groupKey: string): number => {
  const group = getGroupByKey(groupKey);
  if (!group) return 0;
  return group.children.reduce((sum, childKey) => {
    return sum + (props.categorizedStaff[childKey]?.length || 0);
  }, 0);
};

const initializeGroupsOpen = () => {
  let totalStaffCount = 0;
  STAFF_CATEGORIES.forEach((category) => {
    const staffInCategory = props.categorizedStaff[category.key] || [];
    totalStaffCount += staffInCategory.length;
  });
  totalStaffCount += props.uncategorizedStaff.length;

  if (totalStaffCount <= 15) {
    const openGroups: string[] = [];
    CATEGORY_GROUPS.forEach((group) => {
      if (getGroupStaffCount(group.key) > 0) {
        openGroups.push(group.key);
        childCategoriesOpen.value[group.key] = group.children.filter(
          (childKey) => props.categorizedStaff[childKey]?.length > 0,
        );
      }
    });
    if (props.uncategorizedStaff.length > 0) {
      openGroups.push("other");
    }
    groupsOpen.value = openGroups;
  } else {
    groupsOpen.value = [];
    // Keep children collapsed so opening a group doesn't render all staff at once
    CATEGORY_GROUPS.forEach((group) => {
      childCategoriesOpen.value[group.key] = [];
    });
  }
};

watch(
  [() => props.categorizedStaff, () => props.uncategorizedStaff],
  () => {
    initializeGroupsOpen();
  },
  { immediate: true },
);

// When parent requests focus on a specific category (e.g. center-category edge click),
// open the parent group + child panel and scroll to it
watch(
  () => props.focusCategory,
  async (categoryKey) => {
    if (!categoryKey) return;
    const parentGroupKey = CATEGORY_TO_GROUP[categoryKey] || categoryKey;

    // Open the parent group panel
    if (!groupsOpen.value.includes(parentGroupKey)) {
      groupsOpen.value = [...groupsOpen.value, parentGroupKey];
    }

    // Open only the target child category within this group
    childCategoriesOpen.value[parentGroupKey] = [categoryKey];

    // Wait for panels to render, then scroll
    await nextTick();
    await new Promise((resolve) => setTimeout(resolve, 300));

    const container = document.querySelector(
      ".staff-view-content",
    ) as HTMLElement | null;
    if (!container) return;

    const group = getGroupByKey(parentGroupKey);
    if (!group) return;

    const groupPanels = container.querySelectorAll(
      ":scope > .v-expansion-panels > .v-expansion-panel",
    );
    for (const panel of groupPanels) {
      const titleEl = panel.querySelector(".v-expansion-panel-title");
      if (titleEl?.textContent?.includes(group.title_en)) {
        const containerRect = container.getBoundingClientRect();
        const elRect = panel.getBoundingClientRect();
        container.scrollTo({
          top: container.scrollTop + (elRect.top - containerRect.top),
          behavior: "smooth",
        });
        return;
      }
    }
  },
);

const isGroupStaffAllSelected = (groupKey: string): boolean => {
  const ids = props.groupStaffIdsMap[groupKey];
  if (!ids || ids.length === 0) return false;
  return ids.every((id) => selectedStaff.value.has(id));
};

const isGroupStaffSomeSelected = (groupKey: string): boolean => {
  const ids = props.groupStaffIdsMap[groupKey];
  if (!ids || ids.length === 0) return false;
  const selectedCount = ids.filter((id) => selectedStaff.value.has(id)).length;
  return selectedCount > 0 && selectedCount < ids.length;
};

const toggleGroupStaff = (groupKey: string) => {
  const allStaffIds = props.groupStaffIdsMap[groupKey];
  if (!allStaffIds || allStaffIds.length === 0) return;

  const allSelected = allStaffIds.every((id) => selectedStaff.value.has(id));

  if (allSelected) {
    allStaffIds.forEach((id) => selectedStaff.value.delete(id));
  } else {
    allStaffIds.forEach((id) => selectedStaff.value.add(id));
  }

  selectedStaff.value = new Set(selectedStaff.value);
  emit("staffChanged");
};

const toggleCategoryStaff = (categoryKey: string) => {
  const staffIds = props.categoryStaffIdsMap[categoryKey] || [];
  const allSelected = staffIds.every((id: string) =>
    selectedStaff.value.has(id),
  );

  if (allSelected) {
    staffIds.forEach((id: string) => selectedStaff.value.delete(id));
  } else {
    staffIds.forEach((id: string) => selectedStaff.value.add(id));
  }

  selectedStaff.value = new Set(selectedStaff.value);
  emit("staffChanged");
};

const toggleStaff = (staffId: string) => {
  if (selectedStaff.value.has(staffId)) {
    selectedStaff.value.delete(staffId);
  } else {
    selectedStaff.value.add(staffId);
  }
  selectedStaff.value = new Set(selectedStaff.value);
  emit("staffChanged");
};

const isCategoryAllSelected = (categoryKey: string) => {
  const ids = props.categoryStaffIdsMap[categoryKey] || [];
  return (
    ids.length > 0 && ids.every((id: string) => selectedStaff.value.has(id))
  );
};

const isCategorySomeSelected = (categoryKey: string) => {
  const ids = props.categoryStaffIdsMap[categoryKey] || [];
  return ids.some((id: string) => selectedStaff.value.has(id));
};

const getCategoryTitleShort = (categoryKey: string): string => {
  if (categoryKey === "other") return "Other";
  const category = getCategoryByKey(categoryKey);
  return category ? category.title_en : categoryKey;
};
</script>

<style scoped>
.staff-view-content {
  max-height: 600px;
  overflow-y: auto;
}

.cursor-pointer {
  cursor: pointer;
}

.group-color-indicator {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  flex-shrink: 0;
}

.child-category-panels {
  margin-top: 8px;
}

.child-category-panels :deep(.v-expansion-panel) {
  margin-bottom: 4px;
}

.child-panel-title {
  min-height: 40px !important;
  padding: 8px 16px !important;
}

.child-panel-title :deep(.v-expansion-panel-title__icon) {
  margin-inline-start: 8px;
}

/* Staff list: image + name in one flex row */
.staff-list-scrollable {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.staff-list-text {
  min-width: 0;
}

/* Mobile: scroll image + name together as one unit */
@media (max-width: 600px) {
  .staff-view-content :deep(.v-list-item__content) {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    min-width: 0;
  }

  .staff-list-scrollable {
    min-width: max-content;
  }

  .staff-list-text :deep(.v-list-item-title),
  .staff-list-text :deep(.v-list-item-subtitle) {
    white-space: nowrap;
    overflow: visible;
    text-overflow: unset;
  }
}
</style>
