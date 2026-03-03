<template>
  <v-app>
    <AppBar clickable-title />

    <v-main>
      <!-- Loading State -->
      <v-container v-if="loading" class="text-center py-12">
        <v-progress-circular
          indeterminate
          color="primary"
          size="64"
        ></v-progress-circular>
        <p class="text-h6 mt-4">Loading anime details...</p>
      </v-container>

      <!-- Error State -->
      <v-container v-else-if="error" class="text-center py-12">
        <v-icon size="64" color="error">mdi-alert-circle</v-icon>
        <p class="text-h6 mt-4">{{ error }}</p>
        <v-btn color="primary" class="mt-4" to="/">
          Return Home
        </v-btn>
      </v-container>

      <!-- Main Content -->
      <v-container v-else-if="anime" fluid :class="{ 'pt-0': anime.bannerImage }">
        <!-- Banner (hidden) -->
        <v-row v-if="false && anime.bannerImage" class="mt-0">
          <v-col cols="12" class="pa-0">
            <div class="banner-container" @click="bannerExpanded = true">
              <v-img
                :src="anime.bannerImage"
                height="60"
                cover
                gradient="to bottom, rgba(0,0,0,.1), rgba(0,0,0,.7)"
                class="banner-img"
              >
              </v-img>
            </div>
          </v-col>
        </v-row>

        <!-- Banner Fullscreen Overlay -->
        <transition name="banner-fade">
          <div v-if="bannerExpanded" class="banner-overlay" @click="bannerExpanded = false">
            <v-btn icon class="banner-overlay-close" variant="flat" @click.stop="bannerExpanded = false">
              <v-icon size="28" color="white">mdi-close</v-icon>
            </v-btn>
            <img :src="anime.bannerImage" class="banner-overlay-img" draggable="false" @click.stop="bannerExpanded = false" />
          </div>
        </transition>

        <!-- Main Content -->
        <v-row class="mt-4">
          <!-- Left Column: Cover Image & Basic Info -->
          <v-col v-show="!leftColumnCollapsed" cols="12" md="4" lg="3" class="left-column-col">
            <v-card class="sticky-card">
              <v-card-title class="text-h5 text-wrap">
                {{ anime.title }}
              </v-card-title>
              <div class="cover-image-container">
                <v-img
                  :src="anime.coverImage_extraLarge || anime.coverImage_large || anime.coverImage || '/placeholder-anime.jpg'"
                  aspect-ratio="0.7"
                  contain
                ></v-img>

                <!-- Action Buttons Overlay -->
                <div class="action-buttons-bubble">
                  <v-btn
                    icon
                    class="favorite-bubble-btn"
                    :class="{ 'favorited': isFavorited }"
                    @click="toggleFavorite"
                    size="large"
                    variant="flat"
                  >
                    <v-icon size="28" color="white">
                      {{ isFavorited ? 'mdi-heart' : 'mdi-heart-outline' }}
                    </v-icon>
                  </v-btn>
                  <div class="button-divider"></div>
                  <ListButton :anime-id="anime.anilistId" bubble-mode />
                </div>
              </div>
              <v-card-text>
                <div class="mb-3">
                  <v-chip
                    v-if="anime.averageScore"
                    color="primary"
                    class="mr-2 mb-1"
                  >
                    <v-icon start>mdi-star</v-icon>
                    {{ anime.averageScore }}
                  </v-chip>
                  <v-chip v-if="anime.format" class="mr-2 mb-1">
                    {{ formatAnimeFormat(anime.format) }}
                  </v-chip>
                  <v-chip v-if="anime.seasonYear" class="mr-2 mb-1">
                    {{ formatSeasonYear(anime.season, anime.seasonYear) }}
                  </v-chip>
                  <v-chip v-if="anime.status" class="mr-2 mb-1">
                    {{ anime.status }}
                  </v-chip>
                </div>

                <v-divider class="my-3"></v-divider>

                <div class="info-section">
                  <div v-if="anime.studios && anime.studios.length > 0 && anime.studios.some(s => s.name)" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Studios</h4>
                    <div
                      v-for="(studio, idx) in anime.studios.filter(s => s.name)"
                      :key="idx"
                      class="d-inline-flex align-center mr-1 mb-1 gap-1"
                    >
                      <v-chip
                        size="small"
                        class="cursor-pointer"
                        @click="addToAdvancedSearch('studio', studio.name)"
                      >
                        {{ studio.name }}
                        <v-icon end size="large" class="ml-1">mdi-magnify-plus-outline</v-icon>
                      </v-chip>
                      &nbsp;<RouterLink :to="`/studio/${studio.name}`" class="studio-page-link">
                        <v-icon size="14">mdi-open-in-new</v-icon>
                      </RouterLink>
                    </div>
                  </div>

                  <div v-if="anime.episodes" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Episodes</h4>
                    <p>{{ anime.episodes }}</p>
                  </div>

                  <div v-if="anime.duration" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Duration</h4>
                    <p>{{ anime.duration }} min per episode</p>
                  </div>

                  <div v-if="anime.source" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Source</h4>
                    <p>{{ anime.source }}</p>
                  </div>

                  <div v-if="anime.countryOfOrigin" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Country</h4>
                    <p>{{ anime.countryOfOrigin }}</p>
                  </div>

                  <div v-if="filteredGenres.length > 0" class="mb-3">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-1">Genres</h4>
                    <v-chip
                      v-for="genre in filteredGenres"
                      :key="genre"
                      size="small"
                      variant="outlined"
                      class="mr-1 mb-1 cursor-pointer"
                      @click="addToAdvancedSearch('genre', genre)"
                    >
                      {{ genre }}
                      <v-icon end size="large" class="ml-1">
                        mdi-magnify-plus-outline
                      </v-icon>
                    </v-chip>
                  </div>

                  <div v-if="anime.tags && topTags.length > 0" class="mb-3">
                    <div class="d-flex align-center justify-space-between mb-1">
                      <h4 class="text-subtitle-2 text-medium-emphasis mb-0">Tags</h4>
                      <v-btn
                        v-if="topTags.length > 10"
                        variant="text"
                        size="x-small"
                        @click="showAllDetailsTags = !showAllDetailsTags"
                      >
                        {{ showAllDetailsTags ? 'Show Less' : 'Show More' }}
                      </v-btn>
                    </div>
                    <v-chip
                      v-for="tag in displayedDetailsTags"
                      :key="tag.name"
                      size="small"
                      variant="tonal"
                      class="mr-1 mb-1 cursor-pointer"
                      @click="addToAdvancedSearch('tag', tag.name)"
                    >
                      {{ tag.name }}
                      <v-icon end size="large" class="ml-1">
                        mdi-magnify-plus-outline
                      </v-icon>
                    </v-chip>
                  </div>
                </div>

                <v-divider class="my-3"></v-divider>

                <!-- Related Works Grid -->
                <div v-if="relations.length > 0 && !loadingRelations" class="related-works-section">
                  <div class="d-flex align-center justify-space-between mb-2">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-0">Related</h4>
                    <RouterLink
                      v-if="anime.franchise"
                      :to="`/franchise/${encodeURIComponent(anime.franchise.id)}`"
                      class="text-caption text-primary text-decoration-none franchise-link"
                    >
                      <v-icon size="small" class="mr-1">mdi-family-tree</v-icon>
                      {{ anime.franchise.title }} Franchise
                    </RouterLink>
                  </div>
                  <div class="relations-grid">
                    <div
                      v-for="rel in relations"
                      :key="`rel-${rel.anilistId}`"
                      class="relation-item"
                    >
                      <v-tooltip :text="rel.title_english || rel.title || rel.title_romaji" location="top">
                        <template v-slot:activator="{ props }">
                          <v-card
                            v-bind="props"
                            :to="`/anime/${rel.anilistId}`"
                            class="relation-card"
                            :class="{ 'has-long-title': (rel.title_english || rel.title || rel.title_romaji).length > 23 }"
                            elevation="2"
                            hover
                          >
                            <v-img
                              :src="rel.coverImage_large || rel.coverImage_extraLarge || rel.coverImage || '/placeholder-anime.jpg'"
                              aspect-ratio="0.7"
                              :cover="!landscapeRelations.has(String(rel.anilistId))"
                              height="180"
                              :class="{ 'relation-img--landscape': landscapeRelations.has(String(rel.anilistId)) }"
                            ></v-img>
                            <v-card-text class="pa-2">
                              <v-chip
                                :color="getRelationColor(rel.relationType)"
                                size="x-small"
                                class="mb-1"
                                block
                              >
                                {{ formatRelationType(rel.relationType) }}
                              </v-chip>
                              <div class="relation-title-container">
                                <div class="relation-title text-caption">
                                  {{ rel.title_english || rel.title || rel.title_romaji }}
                                </div>
                              </div>
                            </v-card-text>
                          </v-card>
                        </template>
                      </v-tooltip>
                    </div>
                  </div>
                </div>

                <!-- Loading Related Works -->
                <div v-else-if="loadingRelations" class="text-center py-4">
                  <v-progress-circular
                    indeterminate
                    color="primary"
                    size="24"
                  ></v-progress-circular>
                  <p class="text-caption mt-2">Loading related works...</p>
                </div>
              </v-card-text>
            </v-card>
          </v-col>

          <!-- Right Column: Description, Graph -->
          <v-col cols="12" :md="leftColumnCollapsed ? 12 : 8" :lg="leftColumnCollapsed ? 12 : 9" class="right-column-col">
            <div class="d-flex align-center mb-4">
              <v-tabs v-model="activeTab" class="flex-grow-1">
              <v-tab value="overview">Overview</v-tab>
              <v-tab value="graph">Connections</v-tab>
              <v-tab value="search">Search</v-tab>
              <v-tab v-if="sakugabooruPosts.length > 0" value="animation">Animation & Art</v-tab>
              <v-tab v-if="studioComparativeStats && anime.averageScore" value="context">Context</v-tab>
              <v-tab value="links">Links</v-tab>
              </v-tabs>
              <v-btn
                icon
                class="sidebar-toggle-btn d-none d-md-flex ml-2"
                size="x-small"
                variant="text"
                @click="leftColumnCollapsed = !leftColumnCollapsed"
              >
                <v-icon size="18">{{ leftColumnCollapsed ? 'mdi-arrow-expand-right' : 'mdi-arrow-collapse-left' }}</v-icon>
              </v-btn>
            </div>

            <v-window v-model="activeTab">
              <!-- Tab 1: Overview -->
              <v-window-item value="overview">
                <!-- Description -->
                <v-card v-if="anime.description" class="mb-4">
                  <v-card-title>Description</v-card-title>
                  <v-card-text>
                    <p v-html="sanitizeDescription(anime.description)"></p>
                  </v-card-text>
                </v-card>

                <v-card v-if="anime.wikipediaProductionHtml" class="mb-4">
                  <v-card-title class="d-flex align-center justify-space-between">
                    Production Notes
                    <div class="d-flex align-center">
                      <a
                        v-if="anime.wikipediaEn"
                        :href="anime.wikipediaEn"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="text-caption text-medium-emphasis text-decoration-none d-flex align-center"
                      >
                        <v-icon size="14" class="mr-1">mdi-wikipedia</v-icon>
                        Wikipedia
                        <v-icon size="12" class="ml-1">mdi-open-in-new</v-icon>
                      </a>
                      <v-menu :close-on-content-click="false">
                        <template #activator="{ props }">
                          <v-btn v-bind="props" icon size="x-small" variant="text" class="ml-2">
                            <v-icon size="16">mdi-cog</v-icon>
                          </v-btn>
                        </template>
                        <v-card>
                          <v-card-text class="pa-2 px-4 d-flex align-center ga-3">
                            <span class="text-body-2">Show all hyperlinks</span>
                            <v-switch
                              v-model="showAllWikiLinks"
                              density="compact"
                              hide-details
                              class="flex-grow-0"
                            />
                          </v-card-text>
                        </v-card>
                      </v-menu>
                    </div>
                  </v-card-title>
                  <v-card-text>
                    <div class="wikipedia-production-notes" :class="{ 'ref-links-only': !showAllWikiLinks }" v-html="sanitizeWikipediaHtml(anime.wikipediaProductionHtml)"></div>
                  </v-card-text>
                </v-card>
              </v-window-item>

              <!-- Tab 2: Graph & Staff -->
              <v-window-item value="graph">
                <!-- Graph Visualization with Staff -->
                <div class="graph-visualization-wrapper" :class="{ 'graph-loading': !graphVisualizationMounted }">
                <GraphVisualization
                  :anime-id="`${anime.anilistId}`"
                  :staff="anime.staff"
                  :relations="relations"
                  :initial-graph-data="initialGraphData"
                  :anime-data="anime"
                  @vue:mounted="graphVisualizationMounted = true"
                />
            </div>

            <!-- Similar Openings -->
                <v-card v-if="similarOps.length > 0" class="mt-4 mb-4">
                  <v-card-title class="d-flex align-center flex-wrap" style="gap: 8px;">
                    <v-icon start>mdi-music-note</v-icon>
                    Similar Openings
                    <v-btn-toggle
                      v-if="availableOps.length > 1"
                      v-model="selectedOp"
                      mandatory
                      density="compact"
                    >
                      <v-btn v-for="op in availableOps" :key="op" :value="op" size="small">
                        OP{{ op }}
                      </v-btn>
                    </v-btn-toggle>
                    <a
                      v-if="selectedAnimeOp"
                      :href="`https://v.animethemes.moe/${selectedAnimeOp.titleOp}.webm`"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="similar-op-link text-body-2"
                      @click.stop
                    >
                      <v-icon size="16">mdi-play-circle-outline</v-icon>
                      Watch this anime's OP{{ selectedOp }}
                    </a>
                  </v-card-title>
                  <v-card-text style="padding-bottom: 24px;">
                    <v-row>
                      <v-col
                        v-for="op in paginatedSimilarOps"
                        :key="`${op.anilistId}-${op.similarOpNumber}`"
                        cols="6"
                        sm="4"
                        md="3"
                        lg="2"
                        class="similar-op-col"
                      >
                        <AnimeCard :anime="{
                          anilistId: op.anilistId,
                          title: op.title,
                          coverImage_extraLarge: op.coverImage_extraLarge,
                          coverImage_large: op.coverImage_large,
                          averageScore: op.averageScore,
                          format: op.format,
                          seasonYear: op.seasonYear,
                        }" />
                        <a
                          v-if="op.similarTitleOp"
                          :href="`https://v.animethemes.moe/${op.similarTitleOp}.webm`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="similar-op-link similar-op-footer"
                          @click.stop
                        >
                          <v-icon size="14">mdi-play-circle-outline</v-icon>
                          OP{{ op.similarOpNumber }}
                        </a>
                      </v-col>
                    </v-row>
                    <div v-if="similarOpsTotalPages > 1" class="d-flex justify-center mt-4">
                      <v-pagination
                        v-model="similarOpsPage"
                        :length="similarOpsTotalPages"
                        :total-visible="5"
                      ></v-pagination>
                    </div>
                  </v-card-text>
                </v-card>
              </v-window-item>

              <!-- Tab: Search -->
              <v-window-item value="search">
                <!-- Quick Add from Current Anime -->
                <div class="d-flex align-center justify-space-between mb-3">
                  <h3 class="text-subtitle-1 font-weight-bold">Quick Add from "{{ displayTitle }}"</h3>
                  <div class="d-flex gap-2 align-center">
                    <v-chip v-if="loadingFilterMetadata" color="primary" size="small" variant="tonal">
                      <v-progress-circular
                        indeterminate
                        size="16"
                        width="2"
                        class="mr-2"
                      ></v-progress-circular>
                      Loading metadata...
                    </v-chip>
                    <v-chip v-else-if="loadingSearchCounts" color="info" size="small" variant="tonal">
                      <v-progress-circular
                        indeterminate
                        size="16"
                        width="2"
                        class="mr-2"
                      ></v-progress-circular>
                      Updating counts...
                    </v-chip>
                    <v-chip v-else-if="filterMetadataLoaded && searchCountsLoaded" color="success" size="small" variant="tonal">
                      <v-icon start size="small">mdi-lightning-bolt</v-icon>
                      Instant filtering active
                    </v-chip>
                    <v-chip v-else-if="searchCountsLoaded" color="info" size="small" variant="tonal">
                      <v-icon start size="small">mdi-check</v-icon>
                      Counts loaded
                    </v-chip>
                  </div>
                </div>

                <!-- Exclude Related Works Option -->
                <div v-if="relations.length > 0 || (anime.franchise && anime.franchise.id)" class="mb-3">
                  <v-checkbox
                    v-model="excludeRelatedWorks"
                    color="primary"
                    hide-details
                    density="compact"
                    :label="anime.franchise && anime.franchise.id ? 'Exclude franchise & related works' : 'Exclude related works'"
                  >
                  </v-checkbox>
                </div>

                <!-- Studios -->
                <div v-if="anime.studios && anime.studios.length > 0 && anime.studios.some(s => s.name)" class="mb-3">
                  <h4 class="text-subtitle-2 text-medium-emphasis mb-2">Studios</h4>
                  <v-chip
                    v-for="(studio, idx) in anime.studios.filter(s => s.name)"
                    :key="idx"
                    size="default"
                    class="mr-1 mb-1 cursor-pointer"
                    @click="addSearchFilter('studio', studio.name)"
                    :disabled="loadingSearchCounts || searchFilters.studios.includes(studio.name) || searchFilterCounts.studios[studio.name] === 0"
                  >
                    <v-icon start size="large">mdi-plus-circle</v-icon>
                    {{ studio.name }}
                    <span v-if="!loadingSearchCounts && searchFilterCounts.studios[studio.name] !== undefined" class="ml-2 text-caption font-weight-bold">
                      ({{ searchFilterCounts.studios[studio.name] }})
                    </span>
                  </v-chip>
                </div>

                <!-- Genres -->
                <div v-if="filteredGenres.length > 0" class="mb-3">
                  <h4 class="text-subtitle-2 text-medium-emphasis mb-2">Genres</h4>
                  <v-chip
                    v-for="genre in filteredGenres"
                    :key="genre"
                    size="default"
                    variant="outlined"
                    class="mr-1 mb-1 cursor-pointer"
                    @click="addSearchFilter('genre', genre)"
                    :disabled="loadingSearchCounts || searchFilters.genres.includes(genre) || searchFilterCounts.genres[genre] === 0"
                  >
                    <v-icon start size="large">mdi-plus-circle</v-icon>
                    {{ genre }}
                    <span v-if="!loadingSearchCounts && searchFilterCounts.genres[genre] !== undefined" class="ml-2 text-caption font-weight-bold">
                      ({{ searchFilterCounts.genres[genre] }})
                    </span>
                  </v-chip>
                </div>

                <!-- Tags -->
                <div v-if="anime.tags && topTags.length > 0" class="mb-3">
                  <div class="d-flex align-center justify-space-between mb-2">
                    <h4 class="text-subtitle-2 text-medium-emphasis mb-0">Tags</h4>
                    <v-btn
                      v-if="topTags.length > 10"
                      variant="text"
                      size="x-small"
                      @click="showAllSearchTags = !showAllSearchTags"
                    >
                      {{ showAllSearchTags ? 'Show Less' : `Show More (${topTags.length - 10})` }}
                    </v-btn>
                  </div>
                  <v-chip
                    v-for="tag in displayedSearchTags"
                    :key="tag.name"
                    size="default"
                    variant="tonal"
                    class="mr-1 mb-1 cursor-pointer"
                    @click="addSearchFilter('tag', tag.name)"
                    :disabled="loadingSearchCounts || searchFilters.tags.includes(tag.name) || searchFilterCounts.tags[tag.name] === 0"
                  >
                    <v-icon start size="large">mdi-plus-circle</v-icon>
                    {{ tag.name }}
                    <span v-if="!loadingSearchCounts && searchFilterCounts.tags[tag.name] !== undefined" class="ml-2 text-caption font-weight-bold">
                      ({{ searchFilterCounts.tags[tag.name] }})
                    </span>
                  </v-chip>
                </div>

                <v-divider class="my-4"></v-divider>

                <!-- Active Filters -->
                <div v-if="hasSearchFilters" class="mb-4">
                  <h3 class="text-subtitle-1 font-weight-bold mb-3">Active Filters</h3>
                  <div class="d-flex flex-wrap gap-2">
                  <!-- Studio Chips -->
                  <v-chip
                    v-for="studio in searchFilters.studios"
                    :key="`studio-${studio}`"
                    closable
                    @click:close="removeSearchFilter('studio', studio)"
                    color="primary"
                    variant="flat"
                  >
                    <v-icon start>mdi-domain</v-icon>
                    {{ studio }}
                  </v-chip>

                  <!-- Genre Chips -->
                  <v-chip
                    v-for="genre in searchFilters.genres"
                    :key="`genre-${genre}`"
                    closable
                    @click:close="removeSearchFilter('genre', genre)"
                    color="secondary"
                    variant="flat"
                  >
                    <v-icon start>mdi-tag</v-icon>
                    {{ genre }}
                  </v-chip>

                  <!-- Tag Chips -->
                  <v-chip
                    v-for="tag in searchFilters.tags"
                    :key="`tag-${tag}`"
                    closable
                    @click:close="removeSearchFilter('tag', tag)"
                    color="accent"
                    variant="flat"
                  >
                    <v-icon start>mdi-label</v-icon>
                    {{ tag }}
                  </v-chip>

                    <!-- Clear All Button -->
                    <v-btn
                      variant="outlined"
                      size="small"
                      @click="clearSearchFilters"
                      class="ml-2"
                    >
                      Clear All
                    </v-btn>
                  </div>
                </div>

                <v-divider v-if="hasSearchFilters" class="my-4"></v-divider>

                <!-- Search Results Header -->
                <div v-if="hasSearchFilters && !searchLoading">
                  <h2 class="text-h6 font-weight-bold mb-2">
                    Search Results
                  </h2>
                  <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ searchTotal }} items found
                    <span v-if="searchTotalPages > 1"> (Page {{ searchPage }} of {{ searchTotalPages }})</span>
                  </p>
                </div>

                <!-- Loading State -->
                <div v-if="searchLoading" class="text-center py-12">
                  <v-progress-circular
                    indeterminate
                    color="primary"
                    size="64"
                  ></v-progress-circular>
                  <p class="text-h6 mt-4">Searching...</p>
                </div>

                <!-- Search Results Grid -->
                <v-row v-else-if="searchResults.length > 0" class="mt-4">
              <v-col
                v-for="result in searchResults"
                :key="result.anilistId"
                cols="12"
                sm="6"
                md="4"
                lg="2"
              >
                <AnimeCard :anime="result" />
              </v-col>
            </v-row>

                <!-- No Results -->
                <div v-else-if="hasSearchFilters && !searchLoading" class="text-center py-12">
                  <v-icon size="64" color="grey">mdi-emoticon-sad-outline</v-icon>
                  <p class="text-h6 mt-4">No items found</p>
                  <p class="text-body-1 text-medium-emphasis">
                    Try adjusting your filters to find more results
                  </p>
                </div>

                <!-- Initial State -->
                <div v-else-if="!hasSearchFilters && !searchLoading" class="text-center py-12">
                  <v-icon size="64" color="grey">mdi-filter-variant</v-icon>
                  <p class="text-h6 mt-4">Advanced Anime Search</p>
                  <p class="text-body-1 text-medium-emphasis">
                    Click the + icon next to studios, genres, or tags above to add them to your search
                  </p>
                </div>

                <!-- Pagination -->
                <v-row v-if="searchResults.length > 0 && !searchLoading" class="mt-4">
                  <v-col cols="12" class="d-flex justify-center align-center flex-column">
                    <v-pagination
                      v-model="searchPage"
                      :length="searchTotalPages"
                      :total-visible="7"
                      @update:model-value="() => performTabSearch(false)"
                    ></v-pagination>
                    <p class="text-caption text-medium-emphasis mt-2">
                      Page {{ searchPage }} of {{ searchTotalPages }}
                    </p>
                  </v-col>
                </v-row>
              </v-window-item>

              <!-- Tab 3: Animation & Art -->
              <v-window-item value="animation">
                <!-- Sakuga Clips -->
                <SakugaClipsGrid
                  :posts="sakugabooruPosts"
                  :sakugabooru-tag="anime.sakugabooruTag"
                />

                <!-- Promotional Art -->
                <v-card v-if="anime.bannerImage" class="mb-4">
                  <v-card-title>Promotional Art</v-card-title>
                  <v-card-text>
                    <div class="banner-container" @click="bannerExpanded = true">
                      <v-img
                        :src="anime.bannerImage"
                        cover
                        class="banner-img rounded cursor-pointer"
                      />
                    </div>
                  </v-card-text>
                </v-card>
              </v-window-item>

              <!-- Tab 4: Context & Analytics -->
              <v-window-item value="context">
                <!-- Studio Performance Context Section -->
                <v-expansion-panels v-if="studioComparativeStats && anime.averageScore" v-model="studioStatsOpen" class="mt-4 studio-stats-section">
                  <v-expansion-panel>
                    <v-expansion-panel-title>
                      <div class="d-flex align-center justify-space-between w-100">
                        <div class="d-flex align-center">
                          <v-icon start color="primary">mdi-chart-box-outline</v-icon>
                          <span class="text-h6">Studio Performance Context</span>
                        </div>
                    <div class="d-flex align-center gap-2" @click.stop>
                      <!-- Year/Season Toggle -->
                      <v-tooltip :text="studioYearType === 'season' ? 'Switch to Year Only' : 'Switch to Season Breakdown'" location="top">
                        <template v-slot:activator="{ props }">
                          <v-btn
                            v-bind="props"
                            :icon="studioYearType === 'season' ? 'mdi-calendar' : 'mdi-calendar-clock'"
                            size="small"
                            variant="text"
                            @click="toggleStudioYearType"
                            class="context-toggle-btn"
                          >
                          </v-btn>
                        </template>
                      </v-tooltip>
                    </div>
                  </div>
                </v-expansion-panel-title>
                <v-expansion-panel-text>
                  <!-- Loading State -->
                  <div v-if="loadingStudioStats" class="text-center py-8">
                    <v-progress-circular
                      indeterminate
                      color="primary"
                      size="48"
                    ></v-progress-circular>
                    <p class="text-caption mt-3">Loading studio data...</p>
                  </div>

                  <!-- Studio Stats Content -->
                  <div v-else>
                    <!-- Studio Name Header -->
                    <div class="d-flex align-center mb-4">
                      <v-avatar
                        v-if="studioStats?.imageUrl"
                        size="48"
                        rounded="lg"
                        class="mr-3"
                      >
                        <v-img :src="studioStats.imageUrl" :alt="studioComparativeStats.studioName" />
                      </v-avatar>
                      <v-avatar v-else size="48" color="primary" rounded="lg" class="mr-3">
                        <v-icon size="28" color="white">mdi-domain</v-icon>
                      </v-avatar>
                      <h3 class="text-h6">
                        <RouterLink
                          :to="`/studio/${encodeURIComponent(studioComparativeStats.studioName)}`"
                          class="text-decoration-none studio-link"
                        >
                          {{ studioComparativeStats.studioName }}
                        </RouterLink>
                      </h3>
                    </div>

                    <!-- Key Stats Grid -->
                    <v-row class="mb-4">
                      <v-col cols="6" sm="3">
                        <div class="stat-box">
                          <div class="stat-value">
                            #{{ studioComparativeStats.overallRank }}
                            <span class="stat-total">/ {{ studioComparativeStats.totalWithScores }}</span>
                          </div>
                          <div class="stat-label">Overall Rank</div>
                        </div>
                      </v-col>
                      <v-col cols="6" sm="3">
                        <div class="stat-box">
                          <div class="stat-value">
                            {{ anime.averageScore }}
                          </div>
                          <div class="stat-label">
                            Score (avg: {{ studioComparativeStats.avgScore }})
                          </div>
                        </div>
                      </v-col>
                      <v-col cols="6" sm="3" v-if="studioComparativeStats.yearRank > 0">
                        <div class="stat-box">
                          <div class="stat-value">
                            #{{ studioComparativeStats.yearRank }}
                            <span class="stat-total">/ {{ studioComparativeStats.yearTotal }}</span>
                          </div>
                          <div class="stat-label">
                            <span v-if="anime.season">{{ formatSeason(anime.season) }}</span> {{ anime.seasonYear }} Rank
                          </div>
                        </div>
                      </v-col>
                      <v-col cols="6" sm="3" v-if="studioComparativeStats.formatRank > 0">
                        <div class="stat-box">
                          <div class="stat-value">
                            #{{ studioComparativeStats.formatRank }}
                            <span class="stat-total">/ {{ studioComparativeStats.formatTotal }}</span>
                          </div>
                          <div class="stat-label">{{ formatAnimeFormat(anime.format) }} Rank</div>
                        </div>
                      </v-col>
                    </v-row>

                    <!-- Genre Performance -->
                    <div v-if="studioComparativeStats.genreStats.length > 0" class="mb-4">
                      <h4 class="text-subtitle-2 text-medium-emphasis mb-2">
                        Genre Performance
                        <span v-if="selectedStudioGenre" class="text-caption ml-2">(click again to clear)</span>
                      </h4>
                      <div class="d-flex flex-wrap gap-2">
                        <v-tooltip
                          v-for="genreStat in studioComparativeStats.genreStats"
                          :key="genreStat.genre"
                          text="Click to filter timeline"
                          location="top"
                        >
                          <template v-slot:activator="{ props }">
                            <v-chip
                              v-bind="props"
                              size="small"
                              :color="selectedStudioGenre === genreStat.genre ? 'primary' : (genreStat.diff >= 0 ? 'success' : 'error')"
                              :variant="selectedStudioGenre === genreStat.genre ? 'flat' : 'tonal'"
                              @click="selectedStudioGenre = selectedStudioGenre === genreStat.genre ? null : genreStat.genre"
                              class="cursor-pointer genre-chip"
                            >
                              {{ genreStat.genre }}: #{{ genreStat.rank }}/{{ genreStat.total }}
                            </v-chip>
                          </template>
                        </v-tooltip>
                      </div>
                    </div>

                    <!-- Timeline Chart -->
                    <div v-if="selectedStudioGenre && filteredTimelineProductions.length === 0" class="timeline-chart-section">
                      <div class="d-flex align-center justify-space-between mb-3">
                        <h4 class="text-subtitle-2 text-medium-emphasis mb-0">
                          Recent Production Timeline ({{ selectedStudioGenre }} only)
                        </h4>
                        <v-tooltip :text="showCurrentAnimeOnStudioChart ? 'Hide This Anime' : 'Show This Anime'" location="top">
                          <template v-slot:activator="{ props }">
                            <v-btn
                              v-bind="props"
                              :icon="showCurrentAnimeOnStudioChart ? 'mdi-eye' : 'mdi-eye-off'"
                              size="x-small"
                              variant="text"
                              @click="showCurrentAnimeOnStudioChart = !showCurrentAnimeOnStudioChart"
                            >
                            </v-btn>
                          </template>
                        </v-tooltip>
                      </div>
                      <p class="text-caption text-center py-4">
                        No {{ selectedStudioGenre }} productions in timeline
                      </p>
                    </div>
                    <div v-else-if="timelineChartPoints.length > 0 || (!showCurrentAnimeOnStudioChart && filteredTimelineProductions.length > 0)" class="timeline-chart-section">
                      <div class="d-flex align-center justify-space-between mb-3">
                        <h4 class="text-subtitle-2 text-medium-emphasis mb-0">
                          Studio Ratings Timeline - {{ studioYearType === 'season' ? 'By Season' : 'By Year' }}
                          <span v-if="selectedStudioGenre" class="text-caption ml-2">
                            ({{ selectedStudioGenre }} only)
                          </span>
                        </h4>
                        <v-tooltip :text="showCurrentAnimeOnStudioChart ? 'Hide This Anime' : 'Show This Anime'" location="top">
                          <template v-slot:activator="{ props }">
                            <v-btn
                              v-bind="props"
                              :icon="showCurrentAnimeOnStudioChart ? 'mdi-eye' : 'mdi-eye-off'"
                              size="x-small"
                              variant="text"
                              @click="showCurrentAnimeOnStudioChart = !showCurrentAnimeOnStudioChart"
                            >
                            </v-btn>
                          </template>
                        </v-tooltip>
                      </div>
                      <div class="timeline-chart-container">
                        <svg
                          class="timeline-chart-svg"
                          viewBox="0 0 640 280"
                          preserveAspectRatio="xMidYMid meet"
                        >
                          <!-- Grid lines -->
                          <line
                            v-for="i in 5"
                            :key="`grid-${i}`"
                            :x1="60"
                            :y1="30 + (i - 1) * 45"
                            :x2="600"
                            :y2="30 + (i - 1) * 45"
                            stroke="var(--color-primary-faint)"
                            stroke-width="1"
                          />

                          <!-- Y-axis labels (scores) -->
                          <text
                            v-for="i in 5"
                            :key="`ylabel-${i}`"
                            :x="50"
                            :y="35 + (i - 1) * 45"
                            class="chart-label"
                            text-anchor="end"
                          >
                            {{ 100 - (i - 1) * 25 }}
                          </text>

                          <!-- Line path -->
                          <path
                            :d="timelineChartPath"
                            fill="none"
                            stroke="var(--color-primary-border-focus)"
                            stroke-width="2"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          />

                          <!-- Data points -->
                          <circle
                            v-for="(point, index) in timelineChartPoints"
                            :key="`point-${index}`"
                            :cx="point.x"
                            :cy="point.y"
                            :r="point.isCurrentAnime ? 8 : 4"
                            :fill="point.isCurrentAnime ? 'var(--color-primary)' : getScoreColor(point.score)"
                            :class="{
                              'current-anime-point': point.isCurrentAnime,
                              'timeline-point': !point.isCurrentAnime,
                              'aggregated-point': point.isAggregated && studioYearType === 'year'
                            }"
                            :stroke="point.isCurrentAnime ? 'var(--color-accent)' : (point.isAggregated && studioYearType === 'year' ? '#ffc107' : 'none')"
                            :stroke-width="point.isCurrentAnime ? 2 : (point.isAggregated && studioYearType === 'year' ? 2 : 0)"
                            @click="!point.isCurrentAnime && (studioYearType === 'year' ? handleYearNodeClick(point) : router.push(`/anime/${encodeURIComponent(point.anilistId)}`))"
                            @mouseenter="!point.isCurrentAnime && !point.isAggregated && handleTimelinePointHover(point, $event)"
                            @mouseleave="hoveredTimelinePoint = null"
                            :style="{ cursor: point.isCurrentAnime ? 'default' : 'pointer' }"
                          >
                            <title v-if="point.isAggregated && point.animeList">{{ point.animeList.length }} productions (avg: {{ Math.round(point.score) }}%)</title>
                          </circle>

                          <!-- X-axis labels (years or seasons) -->
                          <text
                            v-for="(point, index) in timelineChartPoints"
                            :key="`xlabel-${index}`"
                            v-show="shouldShowTimelineYear(index)"
                            :x="point.x"
                            y="245"
                            class="chart-label chart-label-year"
                            text-anchor="middle"
                          >
                            {{ point.label || point.year }}
                          </text>

                          <!-- Label for current anime -->
                          <text
                            v-if="timelineChartPoints.find(p => p.isCurrentAnime)"
                            :x="timelineChartPoints.find(p => p.isCurrentAnime)?.x"
                            :y="(timelineChartPoints.find(p => p.isCurrentAnime)?.y || 0) - 15"
                            class="current-anime-label"
                            text-anchor="middle"
                          >
                            This anime
                          </text>
                        </svg>
                      </div>

                      <!-- Hover Preview for Timeline Points -->
                      <transition name="fade">
                        <div
                          v-if="hoveredTimelinePoint"
                          class="anime-hover-preview timeline-hover-preview"
                          :style="{
                            left: hoveredTimelinePoint.x + 'px',
                            top: hoveredTimelinePoint.y + 'px'
                          }"
                        >
                          <AnimePreviewCard :anime="hoveredTimelinePoint.point" />
                        </div>
                      </transition>

                      <!-- Year Aggregation Overlay (similar to GraphVisualization) -->
                      <transition name="fade">
                        <div v-if="selectedYearData" class="year-overlay-backdrop" @click="closeYearOverlay"></div>
                      </transition>

                      <transition name="slide-up">
                        <v-card
                          v-if="selectedYearData"
                          class="year-info-overlay"
                          elevation="16"
                        >
                          <!-- Close button -->
                          <v-btn
                            icon="mdi-close"
                            class="overlay-close-btn"
                            @click="closeYearOverlay"
                            size="small"
                            variant="flat"
                            color="primary"
                          ></v-btn>

                          <v-card-title class="text-h6 pa-3">
                            {{ selectedYearData.label }} Productions
                            <span class="text-caption ml-2">({{ selectedYearData.anime.length }} anime)</span>
                          </v-card-title>

                          <v-card-text class="pa-3">
                            <v-row>
                              <v-col
                                v-for="item in selectedYearData.anime"
                                :key="item.anilistId"
                                cols="6"
                                sm="4"
                                md="3"
                              >
                                <AnimeCard
                                  :anime="{
                                    anilistId: item.anilistId,
                                    title: item.title,
                                    coverImage: item.coverImage,
                                    averageScore: item.averageScore,
                                    season: item.season,
                                    seasonYear: item.seasonYear
                                  }"
                                  :show-season="true"
                                  :show-year="false"
                                  :disable-hover="true"
                                />
                              </v-col>
                            </v-row>
                          </v-card-text>
                        </v-card>
                      </transition>
                    </div>
                  </div>
                </v-expansion-panel-text>
              </v-expansion-panel>
            </v-expansion-panels>
              </v-window-item>

              <!-- Tab 5: External Links -->
              <v-window-item value="links">
                <v-card class="mb-4 external-link-card">
                  <v-card-text>
                    <div class="d-flex flex-column gap-3">
                      <a
                        :href="`https://anilist.co/anime/${anime.anilistId}`"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="external-link d-flex align-center text-decoration-none"
                      >
                        <v-icon class="mr-3" color="primary">mdi-open-in-new</v-icon>
                        <div>
                          <div class="text-subtitle-1 font-weight-bold">AniList</div>
                          <div class="text-caption text-medium-emphasis">View on AniList</div>
                        </div>
                      </a>
                      <template v-if="anime.malId">
                        <v-divider />
                        <a
                          :href="`https://myanimelist.net/anime/${anime.malId}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-open-in-new</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">MyAnimeList</div>
                            <div class="text-caption text-medium-emphasis">View on MAL</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.wikipediaEn">
                        <v-divider />
                        <a
                          :href="anime.wikipediaEn"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-wikipedia</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">Wikipedia (EN)</div>
                            <div class="text-caption text-medium-emphasis">English Wikipedia article</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.wikipediaJa">
                        <v-divider />
                        <a
                          :href="anime.wikipediaJa"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-wikipedia</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">Wikipedia (JA)</div>
                            <div class="text-caption text-medium-emphasis">Japanese Wikipedia article</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.wikidataQid">
                        <v-divider />
                        <a
                          :href="`https://www.wikidata.org/wiki/${anime.wikidataQid}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-database-outline</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">Wikidata</div>
                            <div class="text-caption text-medium-emphasis">{{ anime.wikidataQid }}</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.livechartId">
                        <v-divider />
                        <a
                          :href="`https://www.livechart.me/anime/${anime.livechartId}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-chart-line</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">LiveChart</div>
                            <div class="text-caption text-medium-emphasis">View on LiveChart.me</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.tmdbTvId || anime.tmdbMovieId">
                        <v-divider />
                        <a
                          :href="anime.tmdbTvId ? `https://www.themoviedb.org/tv/${anime.tmdbTvId}` : `https://www.themoviedb.org/movie/${anime.tmdbMovieId}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-movie-open-outline</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">TMDB</div>
                            <div class="text-caption text-medium-emphasis">View on The Movie Database</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.tvdbId">
                        <v-divider />
                        <a
                          :href="`https://thetvdb.com/dereferrer/series/${anime.tvdbId}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-television</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">TVDB</div>
                            <div class="text-caption text-medium-emphasis">View on TheTVDB</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.sakugabooruTag">
                        <v-divider />
                        <a
                          :href="`https://www.sakugabooru.com/post?tags=${encodeURIComponent(anime.sakugabooruTag)}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-animation-play-outline</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">Sakugabooru</div>
                            <div class="text-caption text-medium-emphasis">Animation clips and key frames</div>
                          </div>
                        </a>
                      </template>
                      <template v-if="anime.trailer_id && anime.trailer_site">
                        <v-divider />
                        <a
                          :href="anime.trailer_site === 'youtube' ? `https://www.youtube.com/watch?v=${anime.trailer_id}` : `https://www.dailymotion.com/video/${anime.trailer_id}`"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="external-link d-flex align-center text-decoration-none"
                        >
                          <v-icon class="mr-3" color="primary">mdi-play-circle-outline</v-icon>
                          <div>
                            <div class="text-subtitle-1 font-weight-bold">Trailer</div>
                            <div class="text-caption text-medium-emphasis">Watch on {{ anime.trailer_site === 'youtube' ? 'YouTube' : 'Dailymotion' }}</div>
                          </div>
                        </a>
                      </template>
                    </div>
                  </v-card-text>
                </v-card>
              </v-window-item>
            </v-window>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
    <!-- Tutorial Overlay -->
    <TutorialOverlay
      @expand-stats="activeTab = 'context'"
      @expand-search="activeTab = 'search'"
    />
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick, shallowRef, watchEffect } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { api } from '~/utils/api'
import { formatSeason, formatAnimeFormat, formatSeasonYear } from '~/utils/formatters'
import { useTutorial } from '~/composables/useTutorial'
import { useSanitizeHtml } from '~/composables/useSanitizeHtml'
import { useSettings } from '~/composables/useSettings'
import { useAuth } from '~/composables/useAuth'
import { useLoginRequired } from '~/composables/useLoginRequired'
import { useFavorites } from '~/composables/useFavorites'
import { useKeyboardShortcuts } from '~/composables/useKeyboardShortcuts'
import { useFilterMetadata } from '~/composables/useFilterMetadata'

// Tutorial composable
const { tutorialActive, startTutorial } = useTutorial()
const { sanitizeDescription, sanitizeWikipediaHtml: _sanitizeWikipediaHtml } = useSanitizeHtml()

const route = useRoute()
const router = useRouter()
const { includeAdult } = useSettings()
const { getUserId, isAuthenticated } = useAuth()
const { requireLogin } = useLoginRequired()
const { fetchFavorites, isFavorited: checkIsFavorited, toggleFavorite: toggleFavoriteCache, favoritesLoading } = useFavorites()
const animeId = computed(() => route.params.id as string)

const animeResponse = ref<any>(null)
const animeLoading = ref(true)
const fetchError = ref<Error | null>(null)
const graphResponse = ref<any>(null)
const relationsResponse = ref<any>(null)
const relationsLoading = ref(true)

const fetchAnimeData = async () => {
  animeLoading.value = true
  fetchError.value = null
  try {
    animeResponse.value = await api<any>(`/anime/${encodeURIComponent(animeId.value)}`)
  } catch (e: any) {
    fetchError.value = e
  } finally {
    animeLoading.value = false
  }
}

const fetchGraphData = async () => {
  try {
    graphResponse.value = await api<any>(`/graph/${encodeURIComponent(animeId.value)}`)
  } catch (e) {
    console.error('Error fetching graph data:', e)
  }
}

const fetchRelationsData = async () => {
  relationsLoading.value = true
  try {
    relationsResponse.value = await api<any>(`/anime/${encodeURIComponent(animeId.value)}/relations`)
  } catch (e) {
    console.error('Error fetching relations:', e)
  } finally {
    relationsLoading.value = false
  }
}

const anime = computed(() => animeResponse.value?.success ? animeResponse.value.data : null)
const loading = computed(() => animeLoading.value)
const error = computed(() => {
  if (fetchError.value) return fetchError.value.message || 'Failed to load anime details'
  if (animeResponse.value && !animeResponse.value.success) return 'Failed to load anime details'
  return ''
})
const bannerExpanded = ref(false)
const graphVisualizationMounted = ref(false)
const leftColumnCollapsed = ref(false)
const activeTab = ref('graph')
watch(activeTab, () => {
  window.scrollTo({ top: 0, behavior: 'smooth' })
})
const showAllWikiLinks = ref(false)

useKeyboardShortcuts({
  'Escape': () => { if (bannerExpanded.value) bannerExpanded.value = false },
})

// Graph data — passed as prop to GraphVisualization
const initialGraphData = computed(() => graphResponse.value?.success ? graphResponse.value.data : null)

// Favorite state
const isFavorited = computed(() => anime.value ? checkIsFavorited(anime.value.anilistId) : false)

// Relations
const relations = computed(() => relationsResponse.value?.success ? relationsResponse.value.data : [])
const loadingRelations = computed(() => relationsLoading.value)

// Sakugabooru clips
const sakugabooruPosts = computed(() => anime.value?.sakugabooruPosts || [])

// Similar OPs
const similarOps = computed(() => anime.value?.similarOps || [])
const animeOps = computed(() => anime.value?.animeOps || [])
const availableOps = computed(() => [...new Set(similarOps.value.map((op: any) => op.opNumber))].sort())
const selectedOp = ref(1)
const SIMILAR_OPS_PER_PAGE = 12
const similarOpsPage = ref(1)
const filteredSimilarOps = computed(() =>
  similarOps.value.filter((op: any) => op.opNumber === selectedOp.value)
)
const similarOpsTotalPages = computed(() =>
  Math.ceil(filteredSimilarOps.value.length / SIMILAR_OPS_PER_PAGE)
)
const paginatedSimilarOps = computed(() => {
  const start = (similarOpsPage.value - 1) * SIMILAR_OPS_PER_PAGE
  return filteredSimilarOps.value.slice(start, start + SIMILAR_OPS_PER_PAGE)
})
const selectedAnimeOp = computed(() =>
  animeOps.value.find((op: any) => op.opNumber === selectedOp.value)
)

// Reset selected OP when anime changes
watch(() => anime.value?.anilistId, () => {
  selectedOp.value = availableOps.value[0] || 1
  similarOpsPage.value = 1
})
// Reset page when OP selection changes
watch(selectedOp, () => {
  similarOpsPage.value = 1
})

// Track landscape images for relation cards
const landscapeRelations = ref<Set<string>>(new Set())

const checkRelationImage = (anilistId: string, src: string) => {
  if (!src || landscapeRelations.value.has(anilistId)) return
  const img = new Image()
  img.onload = () => {
    if (img.naturalWidth >= img.naturalHeight) {
      landscapeRelations.value = new Set([...landscapeRelations.value, anilistId])
    }
  }
  img.src = src
}

// Check relation images when relations load
watch(relations, (rels) => {
  if (!rels) return
  for (const rel of rels) {
    const src = rel.coverImage_large || rel.coverImage_extraLarge || rel.coverImage
    if (src) checkRelationImage(String(rel.anilistId), src)
  }
}, { immediate: true })

// Studio statistics
const mainStudioName = computed(() => {
  const studios = anime.value?.studios
  if (!studios || studios.length === 0) return null
  const main = studios.find((s: any) => s.name)
  return main?.name || null
})
const studioStatsResponse = ref<any>(null)
const studioStatsLoading = ref(false)

const fetchStudioStats = async () => {
  if (!mainStudioName.value) {
    studioStatsResponse.value = { success: false }
    return
  }
  studioStatsLoading.value = true
  try {
    studioStatsResponse.value = await api<any>(`/studio/${encodeURIComponent(mainStudioName.value)}`)
  } catch (e) {
    console.error('Error fetching studio stats:', e)
    studioStatsResponse.value = { success: false }
  } finally {
    studioStatsLoading.value = false
  }
}

watch(mainStudioName, () => {
  fetchStudioStats()
})

const studioStats = computed(() => studioStatsResponse.value?.success ? studioStatsResponse.value.data : null)
const loadingStudioStats = computed(() => studioStatsLoading.value)

// Year type for studio context (season or year)
const studioYearType = ref<'season' | 'year'>('season')

// Tag display state
const showAllDetailsTags = ref(false)
const showAllSearchTags = ref(false)

// Advanced search expansion state
const advancedSearchOpen = ref<number | undefined>(0)

// Studio stats expansion state
const studioStatsOpen = ref<number | undefined>(0)

// Selected genre for timeline filtering
const selectedStudioGenre = ref<string | null>(null)

// Toggle to show/hide current anime point on studio timeline
const showCurrentAnimeOnStudioChart = ref(true)

// State for year aggregation overlay (studio timeline)
const selectedYearData = ref<{
  year: number
  season?: string
  label: string
  anime: Array<{
    anilistId: string
    title: string
    averageScore: number
    seasonYear: number
    season?: string
    coverImage?: string
  }>
} | null>(null)

// Hover preview state for timeline points
const hoveredTimelinePoint = ref<{ point: any; x: number; y: number } | null>(null)

// Search tab state
const searchFilters = ref<{ studios: string[], genres: string[], tags: string[] }>({
  studios: [],
  genres: [],
  tags: []
})
const excludeRelatedWorks = ref(false)
const searchResults = shallowRef<any[]>([])
const searchLoading = ref(false)
const searchTotal = ref(0)
const searchPage = ref(1)
const searchTotalPages = ref(0)
const searchFilterCounts = ref<any>({ studios: {}, genres: {}, tags: {} })
const loadingSearchCounts = ref(false)
const searchCountsLoaded = ref(false)

// Client-side pagination cache
const ITEMS_PER_PAGE = 12
const PAGES_PER_FETCH = 8 // Fetch 8 pages at a time (96 items)
const searchResultsCache = shallowRef<any[]>([]) // All fetched results
const cachedStartPage = ref(0) // Which page range we have cached
const cachedEndPage = ref(0)

// Client-side filtering using global composable
const {
  filterMetadataLoaded,
  loadingFilterMetadata,
  loadFilterMetadata,
  calculateFilterCounts: calculateFilterCountsFromComposable
} = useFilterMetadata()

const hasSearchFilters = computed(() => {
  return searchFilters.value.studios.length > 0 ||
         searchFilters.value.genres.length > 0 ||
         searchFilters.value.tags.length > 0
})

// Extract related anime IDs for direct relations only
const relatedAnimeIds = computed(() => {
  if (!excludeRelatedWorks.value || relations.value.length === 0) return []
  return relations.value.map((rel: any) => String(rel.anilistId))
})


// Adult content filtering for genres
const ADULT_GENRES = ['Hentai', 'Ecchi']
const filteredGenres = computed(() => {
  if (!anime.value?.genres) return []
  if (includeAdult.value) return anime.value.genres
  return anime.value.genres.filter((g: string) => !ADULT_GENRES.includes(g))
})

const displayTitle = computed(() => {
  if (!anime.value) return ''
  return anime.value.titleEnglish || anime.value.title || anime.value.titleRomaji || 'Unknown Anime'
})

const topTags = computed(() => {
  if (!anime.value?.tags) return []
  const filtered = anime.value.tags.filter((tag: any) => tag && tag.name)

  return filtered
    .sort((a: any, b: any) => (b.rank || 0) - (a.rank || 0))
})

const displayedDetailsTags = computed(() => {
  if (showAllDetailsTags.value) {
    return topTags.value
  }
  return topTags.value.slice(0, 10)
})

const displayedSearchTags = computed(() => {
  if (showAllSearchTags.value) {
    return topTags.value
  }
  return topTags.value.slice(0, 10)
})

// Studio comparative statistics
const studioComparativeStats = computed(() => {
  if (!studioStats.value || !anime.value) return null

  // Only compare to main productions where studio was the main studio
  const allProductions = studioStats.value.mainProductions || []

  if (allProductions.length === 0) return null

  const currentAnimeScore = anime.value.averageScore
  const currentAnimeYear = anime.value.seasonYear
  const currentAnimeFormat = anime.value.format
  const currentAnimeGenres = anime.value.genres || []

  // Filter productions with scores
  const productionsWithScores = allProductions.filter((p: any) => p.anime?.averageScore)

  // Check if current anime is in the studio's production list
  const currentAnimeInList = productionsWithScores.find((p: any) =>
    p.anime?.anilistId === anime.value.anilistId
  )

  // Overall ranking
  const sortedByScore = [...productionsWithScores].sort((a: any, b: any) =>
    (b.anime?.averageScore || 0) - (a.anime?.averageScore || 0)
  )

  let overallRank = 0
  let totalWithScores = productionsWithScores.length

  if (currentAnimeInList) {
    overallRank = sortedByScore.findIndex((p: any) => p.anime?.anilistId === anime.value.anilistId) + 1
  } else if (currentAnimeScore) {
    // Current anime not in list, calculate where it would rank
    overallRank = sortedByScore.filter((p: any) => (p.anime?.averageScore || 0) > currentAnimeScore).length + 1
    totalWithScores = totalWithScores + 1 // Include current anime in total
  }

  // Calculate studio average
  const avgScore = productionsWithScores.reduce((sum: number, p: any) => sum + (p.anime?.averageScore || 0), 0) / productionsWithScores.length
  const scoreDiff = currentAnimeScore ? currentAnimeScore - avgScore : 0

  // Year ranking
  const yearProductions = productionsWithScores.filter((p: any) => p.anime?.seasonYear === currentAnimeYear)
  const yearSorted = [...yearProductions].sort((a: any, b: any) =>
    (b.anime?.averageScore || 0) - (a.anime?.averageScore || 0)
  )
  const yearRank = yearSorted.findIndex((p: any) => p.anime?.anilistId === anime.value.anilistId) + 1
  const yearTotal = yearProductions.length

  // Format ranking
  const formatProductions = productionsWithScores.filter((p: any) => p.anime?.format === currentAnimeFormat)
  const formatSorted = [...formatProductions].sort((a: any, b: any) =>
    (b.anime?.averageScore || 0) - (a.anime?.averageScore || 0)
  )
  const formatRank = formatSorted.findIndex((p: any) => p.anime?.anilistId === anime.value.anilistId) + 1
  const formatTotal = formatProductions.length

  // Genre performance (comparing to ALL studio productions)
  const genreStats = currentAnimeGenres.map((genre: string) => {
    const genreProductions = productionsWithScores.filter((p: any) =>
      p.anime?.genres?.includes(genre)
    )
    const genreAvg = genreProductions.reduce((sum: number, p: any) => sum + (p.anime?.averageScore || 0), 0) / genreProductions.length
    const genreSorted = [...genreProductions].sort((a: any, b: any) =>
      (b.anime?.averageScore || 0) - (a.anime?.averageScore || 0)
    )
    const genreRank = genreSorted.findIndex((p: any) => p.anime?.anilistId === anime.value.anilistId) + 1
    const scoreDiff = currentAnimeScore ? currentAnimeScore - genreAvg : 0

    return {
      genre,
      rank: genreRank,
      total: genreProductions.length,
      diff: scoreDiff // Keep for color coding (green if positive, red if negative)
    }
  }).filter((stat: any) => stat.total > 1) // Only show genres with more than 1 production

  // Timeline: Target 14 productions + current anime (15 total points)
  // Priority: Prefer before, fill with after if needed
  const productionsWithYearAndScore = allProductions.filter((p: any) =>
    p.anime?.seasonYear && p.anime?.averageScore
  )

  const sortedChronologically = [...productionsWithYearAndScore].sort((a: any, b: any) => {
    const yearA = a.anime.seasonYear
    const yearB = b.anime.seasonYear
    if (yearA !== yearB) return yearA - yearB

    // Then by season
    const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }
    const seasonA = seasonOrder[a.anime?.season] || 0
    const seasonB = seasonOrder[b.anime?.season] || 0
    return seasonA - seasonB
  })

  const currentIndex = sortedChronologically.findIndex((p: any) => p.anime?.anilistId === anime.value.anilistId)

  let timelineProductions: any[] = []
  if (currentIndex >= 0) {
    const TARGET_BEFORE = 14
    const productionsBefore = sortedChronologically.slice(0, currentIndex)
    const productionsAfter = sortedChronologically.slice(currentIndex + 1)

    if (productionsBefore.length >= TARGET_BEFORE) {
      // Enough before: take last 14 before + current
      timelineProductions = [
        ...productionsBefore.slice(-TARGET_BEFORE),
        sortedChronologically[currentIndex]
      ]
    } else {
      // Not enough before: take all before + current + fill from after
      const needFromAfter = TARGET_BEFORE - productionsBefore.length
      timelineProductions = [
        ...productionsBefore,
        sortedChronologically[currentIndex],
        ...productionsAfter.slice(0, needFromAfter)
      ]
    }
  }

  // Calculate percentile (what % are you in - e.g., rank 5 of 100 = top 5%)
  const percentile = overallRank > 0 ? Math.round((overallRank / totalWithScores) * 100) : 0

  return {
    studioName: studioStats.value.name,
    overallRank,
    totalWithScores,
    percentile,
    avgScore: Math.round(avgScore),
    scoreDiff: Math.round(scoreDiff),
    yearRank,
    yearTotal,
    formatRank,
    formatTotal,
    genreStats,
    timelineProductions,
    currentIndex
  }
})

// Helper function to create path from points
const createSmoothPath = (points: any[]) => {
  if (points.length === 0) return ''
  if (points.length === 1) return `M ${points[0].x},${points[0].y}`

  let path = `M ${points[0].x},${points[0].y}`

  for (let i = 0; i < points.length - 1; i++) {
    const current = points[i]
    const next = points[i + 1]
    const midX = (current.x + next.x) / 2

    path += ` Q ${current.x},${current.y} ${midX},${(current.y + next.y) / 2}`
    path += ` Q ${next.x},${next.y} ${next.x},${next.y}`
  }

  return path
}

// Filtered timeline based on selected genre
const filteredTimelineProductions = computed(() => {
  if (!studioComparativeStats.value || !studioStats.value) return []

  // If no genre selected, return default timeline
  if (!selectedStudioGenre.value) {
    return studioComparativeStats.value.timelineProductions
  }

  // When genre is selected, show ALL studio main productions with that genre (not limited to default timeline)
  const allProductions = studioStats.value.mainProductions || []

  // Filter to productions with year, score, and matching genre
  return allProductions
    .filter((p: any) => p.anime?.seasonYear && p.anime?.averageScore)
    .filter((production: any) => {
      const genres = production.anime?.genres || []
      return genres.includes(selectedStudioGenre.value)
    })
    .sort((a: any, b: any) => {
      const yearA = a.anime.seasonYear
      const yearB = b.anime.seasonYear
      if (yearA !== yearB) return yearA - yearB

      // Then by season
      const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }
      const seasonA = seasonOrder[a.anime?.season] || 0
      const seasonB = seasonOrder[b.anime?.season] || 0
      return seasonA - seasonB
    })
})

// Timeline chart data (adapted from studio page)
const timelineChartPoints = computed(() => {
  const timeline = filteredTimelineProductions.value
  if (timeline.length === 0) return []

  const chartWidth = 520
  const chartHeight = 180
  const padding = 60
  const bottomPadding = 30

  // If year mode, aggregate productions by year (but keep current anime separate)
  let dataToPlot = timeline
  if (studioYearType.value === 'year') {
    // Group by year and calculate average score, excluding current anime
    const yearMap = new Map<number, { year: number; scores: number[]; productions: any[] }>()
    let currentAnimeProduction: any = null

    timeline.forEach((production: any) => {
      const year = production.anime?.seasonYear
      const score = production.anime?.averageScore
      if (!year || !score) return

      // Keep current anime separate
      if (production.anime?.anilistId === anime.value?.anilistId) {
        currentAnimeProduction = production
        return
      }

      if (!yearMap.has(year)) {
        yearMap.set(year, { year, scores: [], productions: [] })
      }

      const yearData = yearMap.get(year)!
      yearData.scores.push(score)
      yearData.productions.push(production)
    })

    // Convert to array and calculate averages
    const aggregatedData = Array.from(yearMap.values())
      .filter(yd => yd.productions.length > 0)
      .map(yd => ({
        anime: {
          ...yd.productions[0].anime,
          averageScore: yd.scores.reduce((a, b) => a + b, 0) / yd.scores.length,
          seasonYear: yd.year,
          season: null,
          title: `${yd.productions.length} production${yd.productions.length > 1 ? 's' : ''}`,
          anilistId: yd.productions[0].anime?.anilistId
        },
        isMain: true,
        // Store the full anime list for the overlay
        animeList: yd.productions.map((p: any) => ({
          anilistId: p.anime?.anilistId,
          title: p.anime?.title || p.anime?.title_english || p.anime?.title_romaji || 'Unknown',
          averageScore: p.anime?.averageScore,
          seasonYear: p.anime?.seasonYear,
          season: p.anime?.season,
          coverImage: p.anime?.coverImage_large || p.anime?.coverImage_extraLarge || p.anime?.coverImage
        })).sort((a: any, b: any) => {
          // Sort chronologically by season order (Winter -> Spring -> Summer -> Fall)
          const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }
          const seasonA = seasonOrder[a.season] || 0
          const seasonB = seasonOrder[b.season] || 0
          return seasonA - seasonB
        })
      }))

    // Combine current anime with aggregated data, then sort by year
    dataToPlot = currentAnimeProduction
      ? [...aggregatedData, currentAnimeProduction].sort((a: any, b: any) =>
          (a.anime?.seasonYear || 0) - (b.anime?.seasonYear || 0)
        )
      : aggregatedData
  }

  // Separate current anime from other productions
  const currentAnimeProduction = dataToPlot.find((p: any) => p.anime?.anilistId === anime.value?.anilistId)
  const otherProductions = dataToPlot.filter((p: any) => p.anime?.anilistId !== anime.value?.anilistId)

  // Map other productions sequentially
  const points = otherProductions.map((production: any, index: number) => {
    const score = production.anime?.averageScore || 0
    const x = padding + (index / (otherProductions.length - 1 || 1)) * chartWidth
    const y = bottomPadding + chartHeight - (score / 100) * chartHeight

    const season = production.anime?.season
    const year = production.anime?.seasonYear
    const label = studioYearType.value === 'season' && season && year
      ? `${formatSeason(season)} ${year}`
      : year ? `${year}` : ''

    return {
      x,
      y,
      score,
      averageScore: score,
      title: production.anime?.title || 'Unknown',
      anilistId: production.anime?.anilistId,
      coverImage: production.anime?.coverImage_large || production.anime?.coverImage_extraLarge || production.anime?.coverImage,
      format: production.anime?.format || null,
      year,
      seasonYear: year,
      season: studioYearType.value === 'season' ? season : null,
      label,
      isCurrentAnime: false,
      // Include anime list for aggregated nodes (year mode only)
      animeList: production.animeList || null,
      isAggregated: !!(production.animeList && production.animeList.length > 1)
    }
  })

  // Add current anime with horizontal alignment to its time period
  if (currentAnimeProduction && showCurrentAnimeOnStudioChart.value) {
    const currentYear = currentAnimeProduction.anime?.seasonYear
    const currentSeason = currentAnimeProduction.anime?.season
    const currentKey = studioYearType.value === 'season' && currentSeason
      ? `${currentSeason}-${currentYear}`
      : `${currentYear}`

    // Find the x position of a production from the same time period
    let currentX = padding // default to start
    let foundMatch = false
    for (let i = 0; i < otherProductions.length; i++) {
      const prod = otherProductions[i]
      const prodYear = prod.anime?.seasonYear
      const prodSeason = prod.anime?.season
      const prodKey = studioYearType.value === 'season' && prodSeason
        ? `${prodSeason}-${prodYear}`
        : `${prodYear}`

      if (prodKey === currentKey) {
        currentX = points[i].x
        foundMatch = true
        break
      }
    }

    // If no exact match found, interpolate position chronologically
    if (!foundMatch && otherProductions.length > 0) {
      const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }

      // Convert current anime to comparable value
      const currentValue = studioYearType.value === 'season' && currentSeason
        ? currentYear * 10 + (seasonOrder[currentSeason] || 0)
        : currentYear * 10

      // Find where it falls chronologically
      let insertIndex = 0
      for (let i = 0; i < otherProductions.length; i++) {
        const prod = otherProductions[i]
        const prodYear = prod.anime?.seasonYear || 0
        const prodSeason = prod.anime?.season
        const prodValue = studioYearType.value === 'season' && prodSeason
          ? prodYear * 10 + (seasonOrder[prodSeason] || 0)
          : prodYear * 10

        if (prodValue < currentValue) {
          insertIndex = i + 1
        } else {
          break
        }
      }

      // Interpolate x position based on chronological placement
      if (insertIndex === 0) {
        // Before all points
        currentX = padding
      } else if (insertIndex >= otherProductions.length) {
        // After all points
        currentX = padding + chartWidth
      } else {
        // Between two points - interpolate
        const beforeX = points[insertIndex - 1].x
        const afterX = points[insertIndex].x
        currentX = (beforeX + afterX) / 2
      }
    }

    const score = currentAnimeProduction.anime?.averageScore || 0
    const y = bottomPadding + chartHeight - (score / 100) * chartHeight
    const label = studioYearType.value === 'season' && currentSeason && currentYear
      ? `${formatSeason(currentSeason)} ${currentYear}`
      : currentYear ? `${currentYear}` : ''

    points.push({
      x: currentX,
      y,
      score,
      averageScore: score,
      title: currentAnimeProduction.anime?.title || 'Unknown',
      anilistId: currentAnimeProduction.anime?.anilistId,
      coverImage: currentAnimeProduction.anime?.coverImage_large || currentAnimeProduction.anime?.coverImage_extraLarge || currentAnimeProduction.anime?.coverImage,
      format: currentAnimeProduction.anime?.format || null,
      year: currentYear,
      seasonYear: currentYear,
      season: studioYearType.value === 'season' ? currentSeason : null,
      label,
      isCurrentAnime: true,
      animeList: currentAnimeProduction.animeList || null,
      isAggregated: !!(currentAnimeProduction.animeList && currentAnimeProduction.animeList.length > 1)
    })
  }

  return points
})

// Handler for clicking on year nodes (only for year mode in studio context)
const handleYearNodeClick = (point: any) => {
  // Only work in year mode (studio context)
  if (studioYearType.value !== 'year') return

  // Don't handle current anime clicks
  if (point.isCurrentAnime) return

  // Only handle aggregated nodes (year mode with anime list)
  if (!point.animeList || point.animeList.length === 0) return

  // Sort anime chronologically by season (Winter -> Spring -> Summer -> Fall)
  const sortedAnime = [...point.animeList].sort((a: any, b: any) => {
    const seasonOrder: Record<string, number> = { 'WINTER': 1, 'SPRING': 2, 'SUMMER': 3, 'FALL': 4 }
    const seasonA = seasonOrder[a.season] || 0
    const seasonB = seasonOrder[b.season] || 0
    return seasonA - seasonB
  })

  selectedYearData.value = {
    year: point.year,
    season: point.season,
    label: point.label,
    anime: sortedAnime
  }
}

// Close the year overlay
const closeYearOverlay = () => {
  selectedYearData.value = null
}

// Handle timeline point hover
const handleTimelinePointHover = (point: any, event: MouseEvent) => {
  // Get the SVG container to calculate relative position
  const svgContainer = (event.target as SVGElement).closest('.timeline-chart-container')
  if (!svgContainer) return

  const rect = svgContainer.getBoundingClientRect()

  // Calculate position relative to the container
  // Offset to the right and up slightly to avoid overlapping the cursor
  const x = event.clientX - rect.left + 15
  const y = event.clientY - rect.top - 20

  hoveredTimelinePoint.value = {
    point,
    x,
    y
  }
}

const timelineChartPath = computed(() => {
  const points = timelineChartPoints.value
  // Exclude current anime from the line - it will be a floating point
  const linePoints = points.filter(p => !p.isCurrentAnime)

  if (linePoints.length === 0) return ''
  if (linePoints.length === 1) return `M ${linePoints[0].x},${linePoints[0].y}`

  let path = `M ${linePoints[0].x},${linePoints[0].y}`

  for (let i = 0; i < linePoints.length - 1; i++) {
    const current = linePoints[i]
    const next = linePoints[i + 1]
    const midX = (current.x + next.x) / 2

    path += ` Q ${current.x},${current.y} ${midX},${(current.y + next.y) / 2}`
    path += ` Q ${next.x},${next.y} ${next.x},${next.y}`
  }

  return path
})

const getScoreColor = (score: number) => {
  if (score >= 75) return '#4caf50'
  if (score >= 60) return '#8bc34a'
  if (score >= 50) return '#ffc107'
  if (score >= 40) return '#ff9800'
  return '#f44336'
}

// Determine which year/season labels to show to avoid overcrowding
const shouldShowTimelineYear = (index: number) => {
  const points = timelineChartPoints.value
  const point = points[index]

  // Don't show if no year/label
  if (!point?.year && !point?.label) return false

  // Don't show year label for current anime (it has "This anime" label instead)
  if (point.isCurrentAnime) return false

  // Always show first point
  if (index === 0) return true

  // Check if label is different from previous point (handles both year-only and season+year)
  const prevPoint = points[index - 1]
  const currentLabel = point.label || point.year
  const prevLabel = prevPoint?.label || prevPoint?.year
  if (currentLabel === prevLabel) return false

  // Find the last shown year label to check spacing
  const MIN_LABEL_SPACING = 70 // Minimum pixels between year labels
  let lastShownX = points[0].x // First point is always shown

  for (let i = 1; i < index; i++) {
    const p = points[i]
    const pPrev = points[i - 1]
    const pLabel = p.label || p.year
    const pPrevLabel = pPrev?.label || pPrev?.year

    // Track position of labels (including current anime which takes up space even without year label)
    if (p.isCurrentAnime) {
      // Current anime takes up space even without year label
      lastShownX = p.x
    } else if (i > 0 && pLabel !== pPrevLabel) {
      // Check if this label would have been shown
      if (p.x - lastShownX >= MIN_LABEL_SPACING) {
        lastShownX = p.x
      }
    }
  }

  // Show this label only if it's far enough from the last shown label
  return point.x - lastShownX >= MIN_LABEL_SPACING
}

const sanitizeWikipediaHtml = (html: string) => {
  return _sanitizeWikipediaHtml(html, { wikipediaUrl: anime.value?.wikipediaEn })
}

const formatRelationType = (type: string) => {
  if (!type) return 'RELATED'
  return type.replace(/_/g, ' ')
}

const getRelationColor = (type: string) => {
  const colors: Record<string, string> = {
    PREQUEL: 'blue',
    SEQUEL: 'green',
    PARENT: 'purple',
    SIDE_STORY: 'orange',
    ALTERNATIVE: 'pink',
    SPIN_OFF: 'cyan',
    SUMMARY: 'amber',
    COMPILATION: 'lime',
    ADAPTATION: 'teal',
    CHARACTER: 'indigo',
    OTHER: 'grey'
  }
  return colors[type] || 'grey'
}


const toggleStudioYearType = () => {
  studioYearType.value = studioYearType.value === 'season' ? 'year' : 'season'
}

// Client-side filtering function that uses the composable
const calculateFilterCountsClientSide = (checkStudios: string[], checkGenres: string[], checkTags: string[]) => {
  return calculateFilterCountsFromComposable(
    searchFilters.value,
    checkStudios,
    checkGenres,
    checkTags,
    anime.value?.anilistId // Exclude current anime
  )
}

// Search tab functions
const addSearchFilter = (type: 'studio' | 'genre' | 'tag', value: string) => {
  // Trigger metadata load when first filter is added (for instant subsequent counts)
  if (!filterMetadataLoaded.value && !loadingFilterMetadata.value && !hasSearchFilters.value) {
    loadFilterMetadata()
  }

  if (type === 'studio' && !searchFilters.value.studios.includes(value)) {
    searchFilters.value = { ...searchFilters.value, studios: [...searchFilters.value.studios, value] }
  } else if (type === 'genre' && !searchFilters.value.genres.includes(value)) {
    searchFilters.value = { ...searchFilters.value, genres: [...searchFilters.value.genres, value] }
  } else if (type === 'tag' && !searchFilters.value.tags.includes(value)) {
    searchFilters.value = { ...searchFilters.value, tags: [...searchFilters.value.tags, value] }
  }

  // Clear cache when filters change
  searchResultsCache.value = []
  cachedStartPage.value = 0
  cachedEndPage.value = 0
  searchPage.value = 1
  performTabSearch()
}

const removeSearchFilter = (type: 'studio' | 'genre' | 'tag', value: string) => {
  if (type === 'studio') {
    searchFilters.value.studios = searchFilters.value.studios.filter(s => s !== value)
  } else if (type === 'genre') {
    searchFilters.value.genres = searchFilters.value.genres.filter(g => g !== value)
  } else if (type === 'tag') {
    searchFilters.value.tags = searchFilters.value.tags.filter(t => t !== value)
  }

  // Clear cache when filters change
  searchResultsCache.value = []
  cachedStartPage.value = 0
  cachedEndPage.value = 0
  searchPage.value = 1
  performTabSearch()
}

const clearSearchFilters = () => {
  searchFilters.value.studios = []
  searchFilters.value.genres = []
  searchFilters.value.tags = []
  excludeRelatedWorks.value = false
  searchResults.value = []
  searchTotal.value = 0
  searchTotalPages.value = 0
  searchPage.value = 1

  // Clear cache
  searchResultsCache.value = []
  cachedStartPage.value = 0
  cachedEndPage.value = 0

  // Clear URL parameters
  router.replace({
    path: route.path,
    query: {}
  })
}

const fetchSearchFilterCounts = async (force = false, onlyVisible = true, showLoading = true) => {
  if (!anime.value) {
    return
  }


  // Skip if counts already loaded and not forcing a refresh
  if (searchCountsLoaded.value && !force) {
    return
  }

  if (showLoading) {
    loadingSearchCounts.value = true
  }
  try {
    // Collect all studios, genres from the current anime
    const checkStudios = anime.value.studios?.filter((s: any) => s.name).map((s: any) => s.name) || []
    const checkGenres = anime.value.genres || []

    // Only fetch counts for visible tags (first 10) unless showAllSearchTags is true or onlyVisible is false
    let checkTags = anime.value.tags?.filter((t: any) => t.name).map((t: any) => t.name) || []
    if (onlyVisible && !showAllSearchTags.value) {
      checkTags = checkTags.slice(0, 10)
    }

    if (checkStudios.length === 0 && checkGenres.length === 0 && checkTags.length === 0) {
      return
    }

    // HYBRID APPROACH:
    // - If no filters selected yet AND metadata not loaded: use API (fast initial counts)
    // - If filters selected OR metadata loaded: use client-side (instant counts)
    // - Once user adds first filter: trigger metadata load for subsequent instant counts

    if (hasSearchFilters.value || filterMetadataLoaded.value) {
      // Wait for metadata to finish loading if it's currently in progress
      if (loadingFilterMetadata.value && !filterMetadataLoaded.value) {
        await new Promise<void>((resolve) => {
          const unwatch = watch(filterMetadataLoaded, (loaded) => {
            if (loaded) {
              unwatch()
              resolve()
            }
          })
          // Also check immediately in case it finished while we were setting up the watch
          if (filterMetadataLoaded.value) {
            unwatch()
            resolve()
          }
        })
      }

      // Use client-side filtering (instant!)
      const counts = calculateFilterCountsClientSide(checkStudios, checkGenres, checkTags)
      searchFilterCounts.value = {
        studios: { ...searchFilterCounts.value.studios, ...counts.studios },
        genres: { ...searchFilterCounts.value.genres, ...counts.genres },
        tags: { ...searchFilterCounts.value.tags, ...counts.tags }
      }
      searchCountsLoaded.value = true
    } else {
      // Use API for initial counts (when no filters selected yet)
      const params: any = {
        currentStudios: searchFilters.value.studios.join(','),
        currentGenres: searchFilters.value.genres.join(','),
        currentTags: searchFilters.value.tags.join(','),
        checkStudios: checkStudios.join(','),
        checkGenres: checkGenres.join(','),
        checkTags: checkTags.join(','),
        excludeAnimeId: anime.value.anilistId // Exclude current anime from counts
      }

      // Clean up empty params
      Object.keys(params).forEach(key => {
        if (!params[key]) delete params[key]
      })

      const response = await api<any>('/anime/filter-counts', { params })

      if (response.success) {
        // Merge new counts with existing counts
        searchFilterCounts.value = {
          studios: { ...searchFilterCounts.value.studios, ...response.counts.studios },
          genres: { ...searchFilterCounts.value.genres, ...response.counts.genres },
          tags: { ...searchFilterCounts.value.tags, ...response.counts.tags }
        }
        searchCountsLoaded.value = true
      }
    }
  } catch (error) {
    console.error('Error fetching search filter counts:', error)
  } finally {
    if (showLoading) {
      loadingSearchCounts.value = false
    }
  }
}

const performTabSearch = async (recalculateCounts = true) => {
  if (!hasSearchFilters.value) {
    searchResults.value = []
    searchTotal.value = 0
    searchTotalPages.value = 0
    searchResultsCache.value = []
    cachedStartPage.value = 0
    cachedEndPage.value = 0
    return
  }

  // Check if we have the current page in cache
  if (
    searchPage.value >= cachedStartPage.value &&
    searchPage.value <= cachedEndPage.value &&
    searchResultsCache.value.length > 0
  ) {
    // Use cached results for instant pagination
    const startIndex = (searchPage.value - cachedStartPage.value) * ITEMS_PER_PAGE
    const endIndex = startIndex + ITEMS_PER_PAGE
    const results = searchResultsCache.value.slice(startIndex, endIndex)

    searchResults.value = results
    return
  }

  searchLoading.value = true

  // Ensure metadata is loaded for client-side filtering
  if (!filterMetadataLoaded.value) {
    if (!loadingFilterMetadata.value) {
      await loadFilterMetadata()
    } else {
      // Metadata is currently loading - wait for it to finish
      await new Promise<void>((resolve) => {
        const unwatch = watch(filterMetadataLoaded, (loaded) => {
          if (loaded) {
            unwatch()
            resolve()
          }
        })
        // Also check immediately in case it finished while we were setting up the watch
        if (filterMetadataLoaded.value) {
          unwatch()
          resolve()
        }
      })
    }
  }

  try {
    // Calculate which "chunk" of pages to fetch
    // E.g., if user is on page 3, fetch pages 1-5. If on page 7, fetch pages 6-10.
    const chunkIndex = Math.floor((searchPage.value - 1) / PAGES_PER_FETCH)
    const chunkStart = chunkIndex * PAGES_PER_FETCH + 1
    const itemsToFetch = PAGES_PER_FETCH * ITEMS_PER_PAGE
    // apiPage counts pages of `itemsToFetch` items — not client pages of ITEMS_PER_PAGE
    const apiPage = chunkIndex + 1

    const params: any = {
      page: apiPage,
      limit: itemsToFetch,
      excludeAnimeId: anime.value?.anilistId,
      includeAdult: includeAdult.value
    }

    // Add related anime & franchise exclusions if enabled
    if (excludeRelatedWorks.value) {
      // Exclude direct relations
      if (relatedAnimeIds.value.length > 0) {
        params.excludeRelatedAnilistIds = relatedAnimeIds.value.join(',')
      }
      // Exclude entire franchise (let DB handle the filtering)
      if (anime.value?.franchise?.id) {
        params.excludeFranchiseId = anime.value.franchise.id
      }
    }

    if (searchFilters.value.studios.length > 0) {
      params.studios = searchFilters.value.studios.join(',')
    }
    if (searchFilters.value.genres.length > 0) {
      params.genres = searchFilters.value.genres.join(',')
    }
    if (searchFilters.value.tags.length > 0) {
      params.tags = searchFilters.value.tags.join(',')
    }

    const response = await api<any>('/anime/advanced-search', { params })

    if (response.success) {
      // Cache all fetched results
      searchResultsCache.value = response.data
      cachedStartPage.value = chunkStart

      // Calculate how many pages we actually got in this chunk
      const actualPages = Math.ceil(response.data.length / ITEMS_PER_PAGE)
      cachedEndPage.value = chunkStart + actualPages - 1

      // Update total info - calculate totalPages based on client-side ITEMS_PER_PAGE
      // NOT the API's totalPages (which is based on the larger fetch limit)
      searchTotal.value = response.total
      searchTotalPages.value = Math.ceil(response.total / ITEMS_PER_PAGE)

      // Display current page from cache
      const startIndex = (searchPage.value - cachedStartPage.value) * ITEMS_PER_PAGE
      const endIndex = startIndex + ITEMS_PER_PAGE
      const results = searchResultsCache.value.slice(startIndex, endIndex)

      searchResults.value = results
    }
  } catch (error) {
    console.error('Tab search error:', error)
    searchResults.value = []
    searchTotal.value = 0
    searchTotalPages.value = 0
    searchResultsCache.value = []
    cachedStartPage.value = 0
    cachedEndPage.value = 0
  } finally {
    searchLoading.value = false
  }

  // Only recalculate counts when filters change, not on pagination
  if (recalculateCounts) {
    // Wait for metadata to finish loading if it's in progress
    if (loadingFilterMetadata.value) {
      await new Promise<void>((resolve) => {
        const unwatch = watch(loadingFilterMetadata, (loading) => {
          if (!loading) {
            unwatch()
            resolve()
          }
        })
      })
    }

    // Fetch counts after search completes (force refresh to update based on new filters)
    await fetchSearchFilterCounts(true, true)
  }
}

const scrollToWhenStable = (selector: string, timeout = 2000) => {
  const start = Date.now()
  let lastAbsoluteTop = -1
  let stableFrames = 0

  const check = () => {
    if (Date.now() - start > timeout) return
    const el = document.querySelector(selector) as HTMLElement | null
    if (!el || !el.offsetParent) {
      requestAnimationFrame(check)
      return
    }
    // Use document-absolute position (not viewport-relative) so scrolling doesn't affect stability check
    const absoluteTop = el.getBoundingClientRect().top + window.scrollY
    if (Math.abs(absoluteTop - lastAbsoluteTop) < 2) {
      stableFrames++
      if (stableFrames >= 5) {
        window.scrollTo({ top: absoluteTop - 16, behavior: 'smooth' })
        return
      }
    } else {
      stableFrames = 0
    }
    lastAbsoluteTop = absoluteTop
    requestAnimationFrame(check)
  }
  requestAnimationFrame(check)
}

const addToAdvancedSearch = (type: 'studio' | 'genre' | 'tag', value: string) => {
  activeTab.value = 'search'
  addSearchFilter(type, value)
}

// Favorite functions
const toggleFavorite = async () => {
  if (!anime.value) return

  if (!isAuthenticated.value) {
    requireLogin()
    return
  }

  const success = await toggleFavoriteCache(anime.value.anilistId)

  if (!success) {
    // Error already logged in composable
  }
}

// Reset state when anime ID changes
watch(animeId, () => {
  graphVisualizationMounted.value = false
  searchFilters.value.studios = []
  searchFilters.value.genres = []
  searchFilters.value.tags = []
  excludeRelatedWorks.value = false
  searchResults.value = []
  searchTotal.value = 0
  searchTotalPages.value = 0
  searchPage.value = 1
  searchResultsCache.value = []
  cachedStartPage.value = 0
  cachedEndPage.value = 0
  searchCountsLoaded.value = false
})

// When anime data arrives, trigger secondary fetches
watch(anime, async (newAnime) => {
  if (!newAnime) return

  if (isAuthenticated.value) {
    fetchFavorites()
  }

  fetchSearchFilterCounts(false, false, false)

  await loadFilterMetadata()

  nextTick(() => {
    initializeFiltersFromURL()
  })
}, { immediate: true })

// Watch for changes to excludeRelatedWorks to refetch with updated exclusions
watch(excludeRelatedWorks, () => {
  if (hasSearchFilters.value) {
    // Clear cache and refetch from API with updated exclusion list
    searchResultsCache.value = []
    cachedStartPage.value = 0
    cachedEndPage.value = 0
    searchPage.value = 1
    performTabSearch()
  }
})

onMounted(() => {
  // Check for tutorial query parameter
  if (route.query.tutorial === 'true' && animeId.value === '205') {
    // Wait for anime to load before starting tutorial
    const startTutorialWhenReady = async () => {
      // Wait for anime data AND loading to complete
      const waitForPageReady = async () => {
        // Wait for anime to be loaded and loading state to be false
        if (!anime.value || loading.value) {
          const unwatch = watch([anime, loading], ([newAnime, newLoading]) => {
            if (newAnime && !newLoading) {
              unwatch()
              // Give extra time for all elements to render
              setTimeout(() => {
                // Verify key elements exist before starting
                const stickyCard = document.querySelector('.sticky-card')
                const graphWrapper = document.querySelector('.graph-visualization-wrapper')

                if (stickyCard && graphWrapper) {
                  startTutorial()
                } else {
                  // Elements not ready yet, wait a bit more
                  setTimeout(() => {
                    startTutorial()
                  }, 500)
                }
              }, 800)
            }
          })
        } else {
          // Already loaded, wait for render
          await nextTick()
          setTimeout(() => {
            const stickyCard = document.querySelector('.sticky-card')
            const graphWrapper = document.querySelector('.graph-visualization-wrapper')

            if (stickyCard && graphWrapper) {
              startTutorial()
            } else {
              // Elements not ready yet, wait a bit more
              setTimeout(() => {
                startTutorial()
              }, 500)
            }
          }, 800)
        }
      }

      waitForPageReady()
    }
    startTutorialWhenReady()
  }
})

// Initialize filters from URL on mount
const initializeFiltersFromURL = () => {
  const query = route.query

  // Only initialize if we have search-related query params AND they're actually set
  // This prevents accidentally applying old query params from other tabs/pages
  if (!query.studios && !query.genres && !query.tags && !query.searchPage) {
    return
  }

  // Parse URL parameters
  const studios = query.studios ? String(query.studios).split(',').filter(Boolean) : []
  const genres = query.genres ? String(query.genres).split(',').filter(Boolean) : []
  const tags = query.tags ? String(query.tags).split(',').filter(Boolean) : []
  const page = query.searchPage ? parseInt(String(query.searchPage)) : 1

  // Only apply if we have actual filters (not just page number)
  if (studios.length > 0 || genres.length > 0 || tags.length > 0) {
    // Clear any existing state first to avoid conflicts
    searchResultsCache.value = []
    cachedStartPage.value = 0
    cachedEndPage.value = 0
    searchResults.value = []
    searchTotal.value = 0
    searchTotalPages.value = 0

    // Apply filters from URL
    searchFilters.value.studios = studios
    searchFilters.value.genres = genres
    searchFilters.value.tags = tags
    searchPage.value = page

    // Expand the advanced search panel
    advancedSearchOpen.value = 0

    // Wait for anime data to load before performing search
    const waitForAnimeAndSearch = async () => {
      // If anime is already loaded, search immediately
      if (anime.value && animeId.value) {
        await performTabSearch()
        return
      }

      // Otherwise wait for anime to load (with timeout)
      await new Promise<void>((resolve) => {
        const timeout = setTimeout(() => {
          unwatch()
          resolve()
        }, 5000) // 5 second timeout

        const unwatch = watch(anime, (newAnime) => {
          if (newAnime && animeId.value) {
            clearTimeout(timeout)
            unwatch()
            resolve()
          }
        })
      })

      // Perform search if anime loaded
      if (anime.value && animeId.value) {
        await performTabSearch()
      }
    }

    // Call async function but don't await it (runs in background)
    waitForAnimeAndSearch()
  }
}

// Update URL when filters or page change (debounced to avoid excessive updates)
let urlUpdateTimeout: NodeJS.Timeout | null = null
const updateURL = () => {
  const query: any = {}

  if (searchFilters.value.studios.length > 0) {
    query.studios = searchFilters.value.studios.join(',')
  }
  if (searchFilters.value.genres.length > 0) {
    query.genres = searchFilters.value.genres.join(',')
  }
  if (searchFilters.value.tags.length > 0) {
    query.tags = searchFilters.value.tags.join(',')
  }
  if (searchPage.value > 1) {
    query.searchPage = searchPage.value
  }

  // Only update URL if there are filters or we need to clear them
  const hasFilters = Object.keys(query).length > 0
  const hasQueryParams = route.query.studios || route.query.genres || route.query.tags || route.query.searchPage

  if (hasFilters || hasQueryParams) {
    router.replace({
      path: route.path,
      query
    })
  }
}


// Watch filters and update URL
watch(searchFilters, () => {
  if (anime.value) {
    fetchSearchFilterCounts(true, false)
    updateURL()
  }
}, { deep: true })

// Watch page changes and update URL
watch(searchPage, () => {
  if (anime.value && hasSearchFilters.value) {
    updateURL()
  }
})

// Watch for Show More/Less tags to fetch additional counts or clear hidden ones
watch(showAllSearchTags, (showAll) => {
  if (!anime.value) {
    return
  }

  if (showAll) {
    // User clicked "Show More" - tags appear immediately, then load counts after render
    requestAnimationFrame(() => {
      setTimeout(() => {
        fetchSearchFilterCounts(true, false, false)
      }, 0)
    })
  } else if (!showAll) {
    // User clicked "Show Less" - clear counts for tags beyond first 10
    const visibleTags = topTags.value.slice(0, 10).map((t: any) => t.name)
    const newTagCounts: any = {}
    visibleTags.forEach((tag: string) => {
      if (searchFilterCounts.value.tags[tag] !== undefined) {
        newTagCounts[tag] = searchFilterCounts.value.tags[tag]
      }
    })
    searchFilterCounts.value.tags = newTagCounts
  }
})


// Watch for filter metadata to become available and update counts
// This ensures counts switch from API to client-side when metadata loads
watch(filterMetadataLoaded, (isLoaded) => {
  if (isLoaded && anime.value && !hasSearchFilters.value) {
    // Force refresh counts using client-side filtering (instant!)
    // Show loading indicator briefly to provide user feedback
    fetchSearchFilterCounts(true, false, true)
  }
})

// Set page title
watchEffect(() => {
  document.title = (anime.value ? displayTitle.value : 'Loading...') + ' - Anigraph'
})

// Fetch data on mount and when animeId changes
onMounted(() => {
  fetchAnimeData()
  fetchGraphData()
  fetchRelationsData()
  fetchStudioStats()
})

watch(animeId, () => {
  // Clear stale data immediately so components don't flash old content
  graphResponse.value = null
  relationsResponse.value = null
  relationsLoading.value = true

  fetchAnimeData()
  fetchGraphData()
  fetchRelationsData()
})
</script>

<style scoped>

/* Wikipedia production notes */
.wikipedia-production-notes :deep(a) {
  color: rgb(var(--v-theme-primary));
  text-decoration: none;
}
.wikipedia-production-notes :deep(a:hover) {
  text-decoration: underline;
}
.wikipedia-production-notes :deep(h2),
.wikipedia-production-notes :deep(h3),
.wikipedia-production-notes :deep(h4) {
  margin-top: 1em;
  margin-bottom: 0.5em;
}
.wikipedia-production-notes :deep(ul),
.wikipedia-production-notes :deep(ol) {
  padding-left: 1.5em;
  margin-bottom: 0.75em;
}
.wikipedia-production-notes :deep(p) {
  margin-bottom: 0.75em;
}
.wikipedia-production-notes :deep(sup) {
  display: none;
}
.wikipedia-production-notes.ref-links-only :deep(a) {
  color: inherit;
  text-decoration: none;
  pointer-events: none;
  cursor: text;
}
.wikipedia-production-notes.ref-links-only :deep(a:hover) {
  text-decoration: none;
}
.wikipedia-production-notes.ref-links-only :deep(.references a),
.wikipedia-production-notes.ref-links-only :deep(.reflist a) {
  color: rgb(var(--v-theme-primary));
  pointer-events: auto;
  cursor: pointer;
}
.wikipedia-production-notes.ref-links-only :deep(.references a:hover),
.wikipedia-production-notes.ref-links-only :deep(.reflist a:hover) {
  text-decoration: underline;
}

/* Reserve space only while skeleton is shown / graph is hydrating, to prevent CLS.
   Once the graph mounts, the class is removed so collapsing Similar Works moves content up. */
.graph-visualization-wrapper.graph-loading {
  min-height: 700px;
}

/* Skeleton that visually matches the expansion panel structure */
.graph-fallback-skeleton {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity, 0.12));
  border-radius: 4px;
  overflow: hidden;
}

.graph-fallback-header {
  min-height: 48px;
  display: flex;
  align-items: center;
  padding: 0 24px;
  background: rgb(var(--v-theme-surface));
}

.graph-fallback-body {
  background: rgb(var(--v-theme-surface));
}

.v-main {
  overflow-x: clip;
}

.banner-container {
  position: relative;
  cursor: pointer;
  overflow: hidden;
}

.banner-overlay {
  position: fixed;
  inset: 0;
  z-index: 10000;
  background: rgba(var(--color-overlay-rgb), 0.9);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: default;
}

.banner-overlay-close {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 10001;
  background: rgba(var(--color-overlay-rgb), 0.5) !important;
  transition: background 0.2s ease, box-shadow 0.2s ease;
  cursor: pointer;
}

.banner-overlay-close:hover {
  background: rgba(var(--color-text-rgb), 0.15) !important;
  box-shadow: 0 0 0 4px rgba(var(--color-text-rgb), 0.1);
}

.banner-overlay-img {
  max-width: 90vw;
  max-height: 90vh;
  object-fit: contain;
  border-radius: 4px;
  user-select: none;
  -webkit-user-drag: none;
  cursor: pointer;
}


.banner-fade-enter-active,
.banner-fade-leave-active {
  transition: opacity 0.3s ease;
}

.banner-fade-enter-from,
.banner-fade-leave-to {
  opacity: 0;
}

.cover-image-container {
  position: relative;
}

.action-buttons-bubble {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  align-items: center;
  background-color: rgba(var(--color-bg-rgb), 0.85) !important;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(var(--color-primary-rgb), 0.2);
  border-radius: var(--radius-pill);
  padding: 4px;
  box-shadow: var(--shadow-md);
  transition: all var(--transition-base);
  opacity: 0;
  pointer-events: none;
}

.cover-image-container:hover .action-buttons-bubble {
  opacity: 1;
  pointer-events: auto;
}

.action-buttons-bubble:hover {
  border-color: rgba(var(--color-primary-rgb), 0.35);
  box-shadow: var(--shadow-lg);
}

.button-divider {
  width: 1px;
  height: 32px;
  background: rgba(var(--color-primary-rgb), 0.3);
  margin: 0 4px;
}

.favorite-bubble-btn {
  background-color: transparent !important;
  box-shadow: none !important;
  transition: all 0.2s ease;
}

.favorite-bubble-btn:hover {
  background-color: var(--color-primary-faint) !important;
}

.favorite-bubble-btn :deep(.v-icon) {
  color: var(--color-text) !important;
}

.favorite-bubble-btn.favorited :deep(.v-icon) {
  color: var(--color-error) !important;
}

.info-section h4 {
  font-weight: 600;
}

.info-section p {
  margin: 0;
}

.sticky-card {
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
  /* Hide scrollbar for Chrome, Safari and Opera */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE and Edge */
}

.sticky-card::-webkit-scrollbar {
  display: none; /* Chrome, Safari, Opera */
}

/* Only apply sticky on medium+ screens */
@media (max-width: 960px) {
  .sticky-card {
    position: static;
    max-height: none;
    overflow-y: visible;
  }
}

.cursor-pointer {
  cursor: pointer;
}

.franchise-link {
  display: flex;
  align-items: center;
  transition: opacity 0.2s ease;
}

.franchise-link:hover {
  opacity: 0.8;
}

.studio-page-link {
  display: inline-flex;
  align-items: center;
  color: var(--color-text-muted);
  opacity: 0.5;
  line-height: 1;
  text-decoration: none;
  transition: opacity 0.2s ease;
}

.studio-page-link:hover {
  opacity: 1;
}

.genre-chip {
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.genre-chip:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-xs);
}

.gap-2 {
  gap: 8px;
}

/* Relations Grid Styles */
.relations-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.relation-item {
  flex: 0 0 auto;
  width: 130px;
}

.relation-card {
  border-radius: var(--radius-md);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
  overflow: hidden;
}

.relation-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-md) !important;
}

.relation-img--landscape {
  background: rgba(var(--color-overlay-rgb), 0.8);
}

.relation-img--landscape :deep(img) {
  object-fit: contain !important;
}

.relation-title-container {
  height: 36px;
  overflow: hidden;
  position: relative;
}

.relation-title {
  font-size: 0.75rem;
  line-height: 1.2;
  white-space: normal;
  word-wrap: break-word;
}

/* Auto-scroll animation for long titles */
@keyframes scroll-title {
  0%, 10% {
    transform: translateY(0);
  }
  90%, 100% {
    transform: translateY(calc(-100% + 36px));
  }
}

/* Only apply animation when title is long enough to overflow */
.relation-card.has-long-title:hover .relation-title {
  animation: scroll-title 3s ease-in-out 0.5s infinite;
}

/* Studio Statistics Card */
.studio-stats-card {
  background: linear-gradient(135deg, rgba(var(--color-surface-rgb), 0.7) 0%, rgba(var(--color-bg-rgb), 0.9) 100%);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-medium);
  transition: all var(--transition-base);
}

.studio-stats-card:hover {
  border-color: rgba(var(--color-primary-rgb), 0.4);
  box-shadow: var(--shadow-glow);
}

.studio-link {
  color: var(--color-primary);
  transition: color 0.2s ease;
}

.studio-link:hover {
  color: var(--color-accent);
  text-decoration: underline !important;
}

.stat-box {
  background: var(--color-primary-faint);
  border-radius: var(--radius-md);
  padding: 12px;
  text-align: center;
  border: 1px solid var(--color-primary-medium);
  transition: all 0.3s ease;
}

.stat-box:hover {
  background: var(--color-primary-muted);
  border-color: var(--color-primary-strong);
  transform: translateY(-2px);
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--color-primary);
  line-height: 1;
  margin-bottom: 4px;
}

.stat-total {
  font-size: 0.875rem;
  font-weight: 400;
  color: var(--color-text-muted);
}

.stat-label {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.timeline-chart-section {
  position: relative;
  background: rgba(var(--color-bg-rgb), 0.5);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--color-primary-muted);
}

.timeline-chart-container {
  position: relative;
  width: 100%;
  padding: 8px 0;
}

.timeline-chart-svg {
  width: 100%;
  height: auto;
  overflow: visible;
}

/* Hover preview for timeline points */
.anime-hover-preview {
  position: absolute;
  z-index: 30;
  pointer-events: none;
  transform: translate(0, -50%);
}

.timeline-hover-preview {
  /* Additional positioning for timeline context */
}

.chart-label {
  fill: var(--color-text-muted);
  font-size: 12px;
  font-family: system-ui, -apple-system, sans-serif;
}

.chart-label-year {
  font-size: 11px;
  font-weight: 500;
  fill: var(--color-text-muted);
}

.timeline-point {
  cursor: pointer;
  transition: all 0.3s ease;
  filter: drop-shadow(0 2px 4px rgba(var(--color-bg-rgb), 0.4));
}

.timeline-point:hover {
  r: 6;
  filter: drop-shadow(0 3px 6px rgba(var(--color-bg-rgb), 0.5));
}

.current-anime-point {
  cursor: default;
  filter: drop-shadow(0 0 8px var(--color-primary-border-accent));
}

.current-anime-label {
  fill: var(--color-primary);
  font-size: 12px;
  font-weight: 600;
  font-family: system-ui, -apple-system, sans-serif;
  pointer-events: none;
}

.global-timeline-point {
  cursor: default;
  transition: all 0.3s ease;
  filter: drop-shadow(0 2px 4px rgba(var(--color-bg-rgb), 0.4));
}

.global-timeline-point:hover {
  r: 6;
  filter: drop-shadow(0 3px 6px rgba(var(--color-bg-rgb), 0.5));
}

.legend-line {
  width: 24px;
  height: 3px;
  border-radius: 2px;
}

.legend-line-dashed {
  background: repeating-linear-gradient(
    to right,
    var(--color-text-muted) 0px,
    var(--color-text-muted) 4px,
    transparent 4px,
    transparent 8px
  ) !important;
}

.gap-2 {
  gap: 8px;
}

/* Context Toggle Button */
.context-toggle-btn {
  transition: all 0.3s ease;
  opacity: 0.8;
}

.context-toggle-btn:hover {
  opacity: 1;
  transform: scale(1.1);
}

/* Year-by-Year Stats */
.year-stat-box {
  background: var(--color-primary-faint);
  border-radius: 8px;
  padding: 16px;
  text-align: center;
  border: 1px solid var(--color-primary-medium);
  transition: all 0.3s ease;
  height: 100%;
}

.year-stat-box:hover {
  background: var(--color-primary-muted);
  border-color: var(--color-primary-strong);
  transform: translateY(-2px);
}

.year-stat-year {
  font-size: 1.125rem;
  font-weight: 700;
  color: var(--color-primary);
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.year-stat-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.year-stat-rank {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--color-text);
  line-height: 1;
}

.year-stat-total {
  font-size: 0.875rem;
  font-weight: 400;
  color: var(--color-text-muted);
}

.year-stat-label {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* Responsive adjustments for studio stats */
@media (max-width: 600px) {
  .stat-value {
    font-size: 1.25rem;
  }

  .stat-box {
    padding: 8px;
  }

  .timeline-chart-svg {
    height: 200px;
  }

  .year-stat-box {
    padding: 12px;
  }

  .year-stat-year {
    font-size: 1rem;
  }

  .year-stat-rank {
    font-size: 1.125rem;
  }
}

/* Year Aggregation Overlay Styles (matching GraphVisualization) */
.year-overlay-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(var(--color-bg-rgb), 0.8);
  backdrop-filter: blur(4px);
  z-index: 100;
  border-radius: 12px;
}

.year-info-overlay {
  position: absolute;
  inset: 0;
  margin: auto;
  z-index: 101;
  width: 90%;
  max-width: 800px;
  max-height: 80%;
  height: fit-content;
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  background: rgb(var(--v-theme-surface));
}

.year-info-overlay .v-card-title {
  flex-shrink: 0;
}

.year-info-overlay .v-card-text {
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.overlay-close-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
}

.year-anime-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 16px;
  padding: 16px;
}

.year-anime-card {
  position: relative;
  cursor: pointer;
  transition: all 0.3s ease;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(var(--color-surface-rgb), 0.5);
  border: 1px solid var(--color-primary-medium);
}

.year-anime-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-md);
  border-color: rgba(var(--color-primary-rgb), 0.4);
}

.year-anime-score-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  background: rgba(var(--color-primary-rgb), 0.95);
  color: var(--color-text);
  font-weight: 700;
  font-size: 0.75rem;
  padding: 4px 8px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  box-shadow: var(--shadow-xs);
}

.year-anime-cover {
  width: 100%;
  height: auto;
  min-height: 200px;
}

.year-anime-info {
  padding: 12px;
}

.year-anime-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  line-height: 1.3;
  min-height: 2.6em;
}

.year-anime-season {
  color: var(--color-text-muted);
}

/* Aggregated point styles */
.aggregated-point {
  cursor: pointer;
  filter: drop-shadow(0 0 6px rgba(var(--color-primary-rgb), 0.6));
  transition: all 0.3s ease;
}

.aggregated-point:hover {
  r: 6;
  filter: drop-shadow(0 0 10px rgba(var(--color-primary-rgb), 0.8));
}

/* Transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}

.slide-up-enter-from {
  opacity: 0;
  transform: translateY(5%);
}

.slide-up-leave-to {
  opacity: 0;
  transform: translateY(-5%);
}

@media (max-width: 600px) {
  .year-info-overlay {
    width: 95%;
    max-height: 80%;
  }

  .year-anime-grid {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 12px;
    padding: 12px;
  }

  .year-anime-score-badge {
    font-size: 0.65rem;
    padding: 3px 6px;
  }

  .year-anime-cover {
    min-height: 170px;
  }

  .year-anime-info {
    padding: 8px;
  }

  .year-anime-title {
    font-size: 0.8rem;
  }
}

/* Sidebar Toggle Button */
.sidebar-toggle-btn {
  opacity: 0.5;
  transition: opacity 0.2s ease;
}

.sidebar-toggle-btn:hover {
  opacity: 1;
}

/* Left column collapse transition */
.left-column-col {
  transition: all 0.3s ease;
}

/* External link card styles */
.external-link-card {
  overflow: hidden;
}

.external-link {
  padding: 12px 8px;
  border-radius: var(--radius-md);
  color: var(--color-text);
  transition: background 0.2s ease;
}

.external-link:hover {
  background: var(--color-primary-faint);
}

.gap-3 {
  gap: 12px;
}

/* Similar OPs */
.similar-op-col {
  padding-bottom: 28px !important;
}
.similar-op-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px 8px;
  font-size: 0.75rem;
}
.similar-op-link {
  display: flex;
  align-items: center;
  gap: 3px;
  color: rgb(var(--v-theme-primary));
  font-weight: 500;
  text-decoration: none;
}
.similar-op-link:hover {
  text-decoration: underline;
}

</style>
