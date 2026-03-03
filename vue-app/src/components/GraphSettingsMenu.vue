<template>
  <v-menu
    location="bottom end"
    :attach="graphContainer"
    scroll-strategy="close"
  >
    <template v-slot:activator="{ props }">
      <v-btn
        icon
        size="small"
        v-bind="props"
        variant="elevated"
        color="white"
      >
        <v-icon>mdi-cog</v-icon>
      </v-btn>
    </template>
    <v-list
      density="compact"
      width="320"
      style="max-height: min(560px, 70vh); overflow-y: auto"
      class="graph-menu-list"
      @click.stop
    >
      <v-list-subheader>Graph Settings</v-list-subheader>

      <!-- Display -->
      <v-list-item>
        <v-list-item-title class="mb-2">Min Connections</v-list-item-title>
        <v-select
          v-model="minConnections"
          :items="minConnectionsOptions"
          :menu-props="{
            attach: graphContainer,
            scrollStrategy: 'close',
          }"
          name="min-connections"
          density="compact"
          variant="outlined"
          hide-details
        ></v-select>
      </v-list-item>
      <v-list-item>
        <v-list-item-title class="mb-2">Sort Anime By</v-list-item-title>
        <div class="d-flex align-center" style="gap: 8px">
          <v-select
            v-model="graphSortMode"
            :items="[
              { value: 'connections', title: 'Connections' },
              { value: 'rating', title: 'Rating' },
              { value: 'title', title: 'Title' },
            ]"
            :menu-props="{
              attach: graphContainer,
              scrollStrategy: 'close',
            }"
            name="graph-sort-mode"
            density="compact"
            variant="outlined"
            hide-details
            class="flex-grow-1"
          ></v-select>
          <v-btn
            icon
            color="primary"
            variant="tonal"
            density="compact"
            @click="
              graphSortOrder =
                graphSortOrder === 'desc' ? 'asc' : 'desc'
            "
            :title="
              graphSortOrder === 'desc' ? 'Descending' : 'Ascending'
            "
          >
            <v-icon>{{
              graphSortOrder === "desc"
                ? "mdi-arrow-down"
                : "mdi-arrow-up"
            }}</v-icon>
          </v-btn>
        </div>
      </v-list-item>

      <v-divider class="my-2"></v-divider>

      <!-- View Mode -->
      <v-list-item>
        <v-checkbox
          v-model="useCategoryNodes"
          label="Group Staff by Category"
          density="compact"
          hide-details
        ></v-checkbox>
        <v-list-item-subtitle class="text-caption mt-1"
          >Clusters staff into category nodes</v-list-item-subtitle
        >
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hideStaffNodes"
          label="Hide Staff Nodes"
          density="compact"
          hide-details
        ></v-checkbox>
        <v-list-item-subtitle class="text-caption mt-1"
          >Show only anime-to-anime connections</v-list-item-subtitle
        >
      </v-list-item>

      <v-divider class="my-2"></v-divider>

      <!-- Visibility -->
      <v-list-item>
        <v-checkbox
          v-model="sameRoleOnly"
          label="Match category across connections"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hideProducers"
          label="Hide Producers"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hideLonelyStaff"
          label="Hide Lonely Staff"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hideRelatedAnime"
          label="Hide Related Anime"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>

      <v-divider class="my-2"></v-divider>

      <!-- Extras -->
      <v-list-item>
        <v-checkbox
          v-model="showFavorites"
          label="Mark Favorites"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="showFavoriteIcon"
          label="Mark Favorited Anime"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hoverHighlight"
          label="Highlight on Hover"
          density="compact"
          hide-details
        ></v-checkbox>
        <v-list-item-subtitle class="text-caption mt-1"
          >Dim unrelated nodes and edges on hover</v-list-item-subtitle
        >
      </v-list-item>
      <v-list-item>
        <v-checkbox
          v-model="hoverDimmedEdges"
          label="Hover Dimmed Edges"
          density="compact"
          hide-details
        ></v-checkbox>
      </v-list-item>
    </v-list>
  </v-menu>
</template>

<script setup lang="ts">
defineProps<{
  graphContainer: HTMLElement | null;
  minConnectionsOptions: number[];
}>();

const minConnections = defineModel<number>("minConnections", { required: true });
const graphSortMode = defineModel<"connections" | "title" | "rating">("graphSortMode", { required: true });
const graphSortOrder = defineModel<"asc" | "desc">("graphSortOrder", { required: true });
const useCategoryNodes = defineModel<boolean>("useCategoryNodes", { required: true });
const hideStaffNodes = defineModel<boolean>("hideStaffNodes", { required: true });
const sameRoleOnly = defineModel<boolean>("sameRoleOnly", { required: true });
const hideProducers = defineModel<boolean>("hideProducers", { required: true });
const hideLonelyStaff = defineModel<boolean>("hideLonelyStaff", { required: true });
const hideRelatedAnime = defineModel<boolean>("hideRelatedAnime", { required: true });
const showFavorites = defineModel<boolean>("showFavorites", { required: true });
const showFavoriteIcon = defineModel<boolean>("showFavoriteIcon", { required: true });
const hoverHighlight = defineModel<boolean>("hoverHighlight", { required: true });
const hoverDimmedEdges = defineModel<boolean>("hoverDimmedEdges", { required: true });
</script>

<style scoped>
/* Show scrollbar by default on mobile for graph menu lists */
@media (max-width: 600px) {
  .graph-menu-list {
    overflow-y: scroll !important;
  }
}
</style>
