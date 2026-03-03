<template>
  <v-app>
    <AppBar clickable-title />

    <v-main class="studio-page">
      <!-- Loading State -->
      <v-container v-if="loading" class="loading-container">
        <v-progress-circular
          indeterminate
          color="primary"
          size="64"
        ></v-progress-circular>
        <p class="text-h6 mt-4">Loading studio details...</p>
      </v-container>

      <!-- Error State -->
      <v-container v-else-if="error" class="error-container">
        <v-icon size="64" color="error">mdi-alert-circle</v-icon>
        <p class="text-h6 mt-4">{{ error }}</p>
        <v-btn color="primary" class="mt-4" to="/">
          Return Home
        </v-btn>
      </v-container>

      <!-- Studio Details -->
      <v-container v-else-if="studio" fluid class="studio-content">
        <v-row class="mt-4">
          <!-- Left Column: Studio Info & Filters -->
          <v-col cols="12" md="4" lg="3">
            <v-card class="sticky-card">
              <v-card-text class="text-center">
                <!-- Studio Identity -->
                <div class="studio-image-container mb-3">
                  <v-img
                    v-if="studio?.imageUrl"
                    :src="studio.imageUrl"
                    :alt="studio.name"
                    rounded="lg"
                    aspect-ratio="1"
                  >
                    <template #placeholder>
                      <div class="d-flex align-center justify-center fill-height">
                        <v-progress-circular indeterminate color="primary" size="32" />
                      </div>
                    </template>
                  </v-img>
                  <v-avatar v-else size="120" color="primary">
                    <v-icon size="80" color="white">mdi-domain</v-icon>
                  </v-avatar>
                </div>

                <h2 class="text-h5 mb-1">{{ studio?.name || 'Loading...' }}</h2>
                <p class="text-body-2 text-medium-emphasis">
                  {{ totalProductionsCount }} {{ totalProductionsCount === 1 ? 'Production' : 'Productions' }}
                </p>

                <v-divider class="my-3"></v-divider>

                <!-- Quick Stats -->
                <div class="d-flex justify-center">
                  <div class="stats-item">
                    <div class="text-h6 font-weight-bold">{{ totalMainCount }}</div>
                    <div class="text-caption text-medium-emphasis">Main</div>
                  </div>
                  <div class="stats-item">
                    <div class="text-h6 font-weight-bold">{{ totalSupportingCount }}</div>
                    <div class="text-caption text-medium-emphasis">Supporting</div>
                  </div>
                  <div v-if="overallAverageScore > 0" class="stats-item">
                    <div class="text-h6 font-weight-bold text-warning">{{ overallAverageScore.toFixed(1) }}</div>
                    <div class="text-caption text-medium-emphasis">Avg Score</div>
                  </div>
                </div>

                <v-divider class="my-3"></v-divider>

                <!-- View Mode Toggle -->
                <v-btn-toggle
                  v-model="viewMode"
                  mandatory
                  color="primary"
                  variant="outlined"
                  density="compact"
                >
                  <v-btn value="standard" size="small">
                    <v-icon start>mdi-view-grid</v-icon>
                    Overview
                  </v-btn>
                  <v-btn value="analytics" size="small">
                    <v-icon start>mdi-chart-line</v-icon>
                    Analytics
                  </v-btn>
                </v-btn-toggle>
              </v-card-text>

              <!-- Filters - Only in Standard View -->
              <v-expansion-panels
                v-if="viewMode === 'standard'"
                v-model="filtersOpen"
              >
                <v-expansion-panel>
                  <v-expansion-panel-title>
                    <span class="text-h6">Filter by</span>
                  </v-expansion-panel-title>
                  <v-expansion-panel-text>
                    <!-- Active Filters Summary -->
                    <ActiveFilters
                      :has-filters="hasActiveFilters"
                      :summary="`Showing ${activeFilteredCount} of ${activeTotalCount} productions`"
                      @clear-all="clearAllFilters"
                    >
                      <v-chip
                        v-for="genre in selectedGenres"
                        :key="`sel-genre-${genre}`"
                        size="small"
                        closable
                        @click:close="toggleGenreFilter(genre)"
                        color="primary"
                        class="mr-1 mb-1"
                      >
                        {{ genre }}
                      </v-chip>
                      <v-chip
                        v-for="tag in selectedTags"
                        :key="`sel-tag-${tag}`"
                        size="small"
                        closable
                        @click:close="toggleTagFilter(tag)"
                        color="secondary"
                        class="mr-1 mb-1"
                      >
                        {{ tag }}
                      </v-chip>
                      <v-chip
                        v-if="selectedRatingsRange[0] > 0 || selectedRatingsRange[1] < 100"
                        size="small"
                        closable
                        @click:close="selectedRatingsRange = [0, 100]; displayRatingsRange = [0, 100]"
                        color="warning"
                        class="mr-1 mb-1"
                      >
                        <v-icon start size="small">mdi-star</v-icon>
                        {{ selectedRatingsRange[0] }}%-{{ selectedRatingsRange[1] }}%
                      </v-chip>
                      <v-chip
                        v-if="!includeUnratedAnime"
                        size="small"
                        closable
                        @click:close="includeUnratedAnime = true"
                        color="grey"
                        class="mr-1 mb-1"
                      >
                        Rated only
                      </v-chip>
                    </ActiveFilters>

                    <v-divider v-if="hasActiveFilters" class="my-3"></v-divider>

                    <!-- Top Genres -->
                    <FilterSection
                      v-if="studio.genreStats && studio.genreStats.length > 0"
                      :key="`genres-${productionsSubTab}`"
                      title="Top Genres"
                      :items="studio.genreStats"
                      :selected-items="selectedGenres"
                      :filter-counts="filterCounts.genres"
                      :has-active-filters="hasActiveFilters"
                      :limit="8"
                      chip-size="small"
                      chip-variant="tonal"
                      selected-class="selected-genre-filter"
                      section-class="mb-4"
                      @toggle="toggleGenreFilter"
                    />

                    <!-- Top Tags -->
                    <FilterSection
                      v-if="studio.tagStats && studio.tagStats.length > 0"
                      :key="`tags-${productionsSubTab}`"
                      title="Common Themes"
                      :items="studio.tagStats"
                      :selected-items="selectedTags"
                      :filter-counts="filterCounts.tags"
                      :has-active-filters="hasActiveFilters"
                      :limit="8"
                      chip-size="small"
                      chip-variant="tonal"
                      selected-class="selected-tag-filter"
                      section-class="mb-3"
                      @toggle="toggleTagFilter"
                    />

                    <!-- Rating Range -->
                    <div class="mt-3">
                      <h4 class="text-subtitle-2 text-medium-emphasis mb-2">
                        <v-icon size="small" color="warning" class="mr-1">mdi-star</v-icon>
                        Rating
                      </h4>
                      <v-range-slider
                        v-model="displayRatingsRange"
                        :min="0"
                        :max="100"
                        :step="1"
                        thumb-label
                        color="warning"
                        hide-details
                        @end="onRatingsRangeEnd"
                      ></v-range-slider>
                      <div class="d-flex justify-space-between">
                        <span class="text-caption text-medium-emphasis">{{ displayRatingsRange[0] }}%</span>
                        <span class="text-caption text-medium-emphasis">{{ displayRatingsRange[1] }}%</span>
                      </div>
                      <v-checkbox
                        v-model="includeUnratedAnime"
                        label="Include unrated"
                        density="compact"
                        hide-details
                        color="warning"
                        class="mt-2"
                      ></v-checkbox>
                    </div>
                  </v-expansion-panel-text>
                </v-expansion-panel>
              </v-expansion-panels>
            </v-card>
          </v-col>

          <!-- Right Column: Main Content -->
          <v-col cols="12" md="8" lg="9">
            <v-tabs v-model="activeTab">
              <v-tab value="works">Works</v-tab>
              <v-tab v-if="studio.wikipediaContentHtml || studio.description" value="about">About</v-tab>
            </v-tabs>

            <v-window v-model="activeTab" :touch="false">
              <!-- Works Tab -->
              <v-window-item value="works">
            <!-- Standard View -->
            <div v-if="viewMode === 'standard'" class="standard-section">
              <!-- Timeline Title with Dropdown -->
              <ViewToolbar
                v-model:card-size="cardSize"
                v-model:sort-by="sortBy"
                v-model:year-markers-enabled="showYearMarkers"
                :sort-order="sortOrder"
                :sort-options="sortOptions"
                :show-sort="true"
                :show-year-markers="true"
                @toggle-sort-order="toggleSortOrder"
              >
                <template #left>
                  <v-icon color="primary">mdi-timeline-clock</v-icon>
                  <v-select
                    v-model="productionsSubTab"
                    :items="[
                      { title: 'All Productions', value: 'all' },
                      { title: 'Main Studio', value: 'main' },
                      { title: 'Supporting', value: 'supporting' }
                    ]"
                    variant="outlined"
                    density="compact"
                    hide-details
                    style="max-width: 200px;"
                    class="timeline-select"
                  ></v-select>
                  <v-chip size="small" variant="flat" color="primary" v-if="productionsSubTab === 'all'">
                    <span v-if="hasActiveFilters">{{ filteredProductionsCount }}/</span>{{ totalProductionsCount }}
                  </v-chip>
                  <v-chip size="small" variant="flat" color="primary" v-else-if="productionsSubTab === 'main'">
                    <span v-if="hasActiveFilters">{{ filteredMainCount }}/</span>{{ totalMainCount }}
                  </v-chip>
                  <v-chip size="small" variant="flat" color="primary" v-else-if="productionsSubTab === 'supporting'">
                    <span v-if="hasActiveFilters">{{ filteredSupportingCount }}/</span>{{ totalSupportingCount }}
                  </v-chip>
                </template>
              </ViewToolbar>

              <!-- Productions -->
              <v-row>
                <template v-for="(item, idx) in activeProductions" :key="item.isYearMarker ? `year-${item.year}` : item.isSpacer ? `spacer-${idx}` : item.anime?.anilistId || idx">
                  <!-- Spacer -->
                  <v-col v-if="item.isSpacer && showYearMarkers" cols="12" sm="6" md="4" :lg="cardColSize" class="spacer-col">
                    <YearCard spacer />
                  </v-col>
                  <!-- Year Marker -->
                  <v-col v-else-if="item.isYearMarker && showYearMarkers" cols="12" sm="6" md="4" :lg="cardColSize" class="year-marker-col">
                    <YearCard :year="item.year" :count="item.count" count-label="production" :continued="item.continued" />
                  </v-col>
                  <!-- Production Card -->
                  <v-col
                    v-else-if="!item.isSpacer && !item.isYearMarker"
                    cols="12"
                    sm="6"
                    md="4"
                    :lg="cardColSize"
                  >
                    <AnimeCard
                      :anime="item.anime"
                      :show-season="true"
                      :show-year="!showYearMarkers || sortBy === 'score'"
                    />
                  </v-col>
                </template>
              </v-row>

              <!-- Pagination -->
              <v-row v-if="totalPages > 1" class="mt-4">
                <v-col cols="12" class="d-flex justify-center">
                  <v-pagination
                    v-model="currentPage"
                    :length="totalPages"
                    :total-visible="7"
                  ></v-pagination>
                </v-col>
              </v-row>
            </div>

            <!-- Analytics View -->
            <div v-if="viewMode === 'analytics'" class="analytics-section">
              <!-- Statistics Cards -->
              <v-row class="mb-6">
                <v-col cols="12" sm="6" md="4">
                  <v-card class="stat-card">
                    <v-card-text class="text-center pa-6">
                      <v-icon size="48" color="warning" class="mb-3">mdi-star</v-icon>
                      <div class="text-h3 font-weight-bold mb-2">
                        {{ overallAverageScore > 0 ? overallAverageScore.toFixed(1) : 'N/A' }}
                      </div>
                      <div class="text-subtitle-1 text-medium-emphasis">Average Score</div>
                      <div class="text-caption text-medium-emphasis mt-1">
                        Across {{ totalProductionsCount }} productions
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-card class="stat-card">
                    <v-card-text class="text-center pa-6">
                      <v-icon size="48" color="primary" class="mb-3">mdi-television-classic</v-icon>
                      <div class="text-h3 font-weight-bold mb-2">{{ totalProductionsCount }}</div>
                      <div class="text-subtitle-1 text-medium-emphasis">Total Productions</div>
                      <div class="text-caption text-medium-emphasis mt-1">
                        Main: {{ totalMainCount }} • Supporting: {{ totalSupportingCount }}
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-card class="stat-card">
                    <v-card-text class="text-center pa-6">
                      <v-icon size="48" color="success" class="mb-3">mdi-filmstrip</v-icon>
                      <div class="text-h3 font-weight-bold mb-2">{{ formatStats.length }}</div>
                      <div class="text-subtitle-1 text-medium-emphasis">Format Types</div>
                      <div class="text-caption text-medium-emphasis mt-1">
                        {{ formatStats[0]?.name || 'N/A' }} is most common
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>
              </v-row>

              <v-row>
                <!-- Genre Distribution -->
                <v-col cols="12" lg="6">
                  <v-card class="analytics-card">
                    <v-card-title class="d-flex align-center analytics-card-title">
                      <v-icon start color="primary">mdi-chart-pie</v-icon>
                      Genre Distribution
                    </v-card-title>
                    <v-card-text>
                      <div v-if="studio.genreStats && studio.genreStats.length > 0">
                        <div
                          v-for="(genre, index) in studio.genreStats.slice(0, 10)"
                          :key="genre.name"
                          class="genre-bar-item mb-3"
                        >
                          <div class="d-flex align-center justify-space-between mb-1">
                            <span class="text-subtitle-2">{{ genre.name }}</span>
                            <span class="text-caption text-medium-emphasis">
                              {{ genre.count }} ({{ getGenrePercentage(genre.count) }}%)
                            </span>
                          </div>
                          <v-progress-linear
                            :model-value="getGenrePercentage(genre.count)"
                            :color="getAnalyticsGenreColor(index)"
                            height="10"
                            rounded
                            class="analytics-progress-bar"
                          ></v-progress-linear>
                        </div>
                      </div>
                      <div v-else class="text-center text-medium-emphasis py-4">
                        No genre data available
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>

                <!-- Top Tags -->
                <v-col cols="12" lg="6">
                  <v-card class="analytics-card">
                    <v-card-title class="d-flex align-center analytics-card-title">
                      <v-icon start color="secondary">mdi-label-multiple</v-icon>
                      Top Tags
                    </v-card-title>
                    <v-card-text>
                      <div v-if="studio.tagStats && studio.tagStats.length > 0">
                        <div
                          v-for="(tag, index) in studio.tagStats.slice(0, 10)"
                          :key="tag.name"
                          class="tag-bar-item mb-3"
                        >
                          <div class="d-flex align-center justify-space-between mb-1">
                            <span class="text-subtitle-2">{{ tag.name }}</span>
                            <span class="text-caption text-medium-emphasis">
                              {{ tag.count }} productions
                            </span>
                          </div>
                          <v-progress-linear
                            :model-value="getTagPercentage(tag.count)"
                            :color="getAnalyticsTagColor(index)"
                            height="10"
                            rounded
                            class="analytics-progress-bar"
                          ></v-progress-linear>
                        </div>
                      </div>
                      <div v-else class="text-center text-medium-emphasis py-4">
                        No tag data available
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>

                <!-- Format Distribution -->
                <v-col cols="12" lg="6">
                  <v-card class="analytics-card">
                    <v-card-title class="d-flex align-center analytics-card-title">
                      <v-icon start color="info">mdi-filmstrip</v-icon>
                      Format Distribution
                    </v-card-title>
                    <v-card-text>
                      <div v-if="formatStats.length > 0">
                        <div
                          v-for="(format, index) in formatStats"
                          :key="format.name"
                          class="format-bar-item mb-3"
                        >
                          <div class="d-flex align-center justify-space-between mb-1">
                            <span class="text-subtitle-2">{{ format.name }}</span>
                            <span class="text-caption text-medium-emphasis">
                              {{ format.count }} ({{ getFormatPercentage(format.count) }}%)
                            </span>
                          </div>
                          <v-progress-linear
                            :model-value="getFormatPercentage(format.count)"
                            :color="getAnalyticsFormatColor(index)"
                            height="10"
                            rounded
                            class="analytics-progress-bar"
                          ></v-progress-linear>
                        </div>
                      </div>
                      <div v-else class="text-center text-medium-emphasis py-4">
                        No format data available
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>

                <!-- Average Score Trends -->
                <v-col cols="12" lg="6">
                  <v-card class="analytics-card">
                    <v-card-title class="d-flex align-center analytics-card-title">
                      <v-icon start color="warning">mdi-chart-line</v-icon>
                      Average Score Trends
                    </v-card-title>
                    <v-card-text>
                      <div v-if="scoreTrendsByYear.length > 0" class="score-trends-chart-container">
                        <svg
                          class="score-trends-svg"
                          viewBox="0 0 600 340"
                          preserveAspectRatio="xMidYMid meet"
                        >
                          <!-- Grid lines -->
                          <line
                            v-for="i in 5"
                            :key="`grid-${i}`"
                            :x1="60"
                            :y1="30 + (i - 1) * 60"
                            :x2="580"
                            :y2="30 + (i - 1) * 60"
                            stroke="var(--color-primary-faint)"
                            stroke-width="1"
                          />

                          <!-- Y-axis labels (scores) -->
                          <text
                            v-for="i in 5"
                            :key="`ylabel-${i}`"
                            :x="50"
                            :y="35 + (i - 1) * 60"
                            class="chart-label"
                            text-anchor="end"
                          >
                            {{ 100 - (i - 1) * 20 }}
                          </text>

                          <!-- Line path -->
                          <path
                            :d="scoreLinePath"
                            fill="none"
                            stroke="url(#scoreGradient)"
                            stroke-width="3"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            class="score-line"
                          />

                          <!-- Area under the line -->
                          <path
                            :d="scoreAreaPath"
                            fill="url(#scoreAreaGradient)"
                            opacity="0.2"
                          />

                          <!-- Data points -->
                          <circle
                            v-for="(point, index) in scoreLinePoints"
                            :key="`point-${index}`"
                            :cx="point.x"
                            :cy="point.y"
                            r="5"
                            :fill="getScoreColor(point.score)"
                            class="score-point"
                          >
                            <title>{{ point.year }}: {{ point.score }}%</title>
                          </circle>

                          <!-- X-axis labels (years) -->
                          <text
                            v-for="(point, index) in scoreLinePoints"
                            :key="`xlabel-${index}`"
                            v-show="shouldShowYearLabel(index)"
                            :x="point.x"
                            y="290"
                            class="chart-label chart-label-year"
                            text-anchor="end"
                            :transform="`rotate(-45, ${point.x}, 290)`"
                          >
                            {{ point.year }}
                          </text>

                          <!-- Gradients -->
                          <defs>
                            <linearGradient id="scoreGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                              <stop offset="0%" style="stop-color:var(--color-primary);stop-opacity:1" />
                              <stop offset="100%" style="stop-color:var(--color-accent);stop-opacity:1" />
                            </linearGradient>
                            <linearGradient id="scoreAreaGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                              <stop offset="0%" style="stop-color:var(--color-primary);stop-opacity:0.4" />
                              <stop offset="100%" style="stop-color:var(--color-primary);stop-opacity:0" />
                            </linearGradient>
                          </defs>
                        </svg>

                        <!-- Legend -->
                        <div class="chart-legend mt-4">
                          <div class="d-flex align-center justify-center gap-3 flex-wrap">
                            <div class="d-flex align-center">
                              <div class="legend-dot" style="background-color: var(--color-success)"></div>
                              <span class="text-caption">Excellent (75+)</span>
                            </div>
                            <div class="d-flex align-center">
                              <div class="legend-dot" style="background-color: var(--color-score)"></div>
                              <span class="text-caption">Good (50-74)</span>
                            </div>
                            <div class="d-flex align-center">
                              <div class="legend-dot" style="background-color: var(--color-error)"></div>
                              <span class="text-caption">Below Average (&lt;50)</span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <div v-else class="text-center text-medium-emphasis py-4">
                        No score trend data available
                      </div>
                    </v-card-text>
                  </v-card>
                </v-col>
              </v-row>
            </div>
              </v-window-item>

              <!-- About Tab -->
              <v-window-item value="about">
                <v-card class="mb-4">
                  <v-card-title class="d-flex align-center justify-space-between flex-wrap ga-2">
                    <div v-if="studio.description && studio.wikipediaContentHtml" class="d-flex align-center ga-2">
                      <v-chip
                        size="small"
                        :variant="aboutMode === 'full' ? 'flat' : 'text'"
                        :color="aboutMode === 'full' ? 'primary' : undefined"
                        class="about-toggle-chip"
                        @click="aboutMode = 'full'"
                      >
                        Full Article
                      </v-chip>
                      <v-chip
                        size="small"
                        :variant="aboutMode === 'short' ? 'flat' : 'text'"
                        :color="aboutMode === 'short' ? 'primary' : undefined"
                        class="about-toggle-chip"
                        @click="aboutMode = 'short'"
                      >
                        Summary
                      </v-chip>
                    </div>
                    <span v-else>About</span>
                    <div v-if="aboutMode === 'full' && studio.wikipediaContentHtml" class="d-flex align-center">
                      <a
                        v-if="studio.wikipediaEn"
                        :href="studio.wikipediaEn"
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
                    <!-- Short description -->
                    <div v-if="aboutMode === 'short'" class="wikipedia-content">
                      <p>{{ studio.description }}</p>
                    </div>
                    <!-- Full Wikipedia content -->
                    <div v-else class="wikipedia-content" :class="{ 'ref-links-only': !showAllWikiLinks }" v-html="sanitizeWikipediaHtml(studio.wikipediaContentHtml)"></div>
                  </v-card-text>
                </v-card>
              </v-window-item>
            </v-window>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/utils/api'
import { filterAdultContent } from '@/utils/contentFilters'
import { sortByYearAndScore } from '@/utils/sorting'
import { flattenWithYearMarkers, paginateWithYearMarkers } from '@/utils/yearMarkers'
import { useSanitizeHtml } from '@/composables/useSanitizeHtml'
import { useSettings } from '@/composables/useSettings'
import { useCardSize } from '@/composables/useCardSize'
import { useSortable } from '@/composables/useSortable'

const { sanitizeWikipediaHtml: _sanitizeWikipediaHtml } = useSanitizeHtml()
const route = useRoute()
const router = useRouter()
const { includeAdult } = useSettings()
const studioId = computed(() => route.params.id as string)

// Manual fetch replacing useAsyncData
const studioResponse = ref<any>(null)
const pending = ref(false)
const fetchError = ref<string | null>(null)

async function fetchStudio() {
  if (!studioId.value) return
  pending.value = true
  fetchError.value = null
  try {
    studioResponse.value = await api<any>(`/studio/${encodeURIComponent(studioId.value)}`)
  } catch (e: any) {
    fetchError.value = e.message
  } finally {
    pending.value = false
  }
}

onMounted(fetchStudio)
watch(studioId, fetchStudio)

const studio = computed(() => studioResponse.value?.success ? studioResponse.value.data : null)
const loading = computed(() => pending.value)
const error = computed(() => {
  if (fetchError.value) return fetchError.value || 'Failed to load studio details'
  if (studioResponse.value && !studioResponse.value.success) return 'Failed to load studio details'
  return ''
})

const activeTab = ref('works')
const showAllWikiLinks = ref(false)
const aboutMode = ref<'short' | 'full'>('full')

// If no description, force full mode; if no wikipedia, force short
watch(() => studio.value, (s) => {
  if (s && !s.description && s.wikipediaContentHtml) aboutMode.value = 'full'
  else if (s && s.description && !s.wikipediaContentHtml) aboutMode.value = 'short'
}, { immediate: true })

const sanitizeWikipediaHtml = (html: string) => {
  return _sanitizeWikipediaHtml(html, { wikipediaUrl: studio.value?.wikipediaEn, stripTopHeading: false })
}

// Initialize view mode from URL query or default to 'standard'
const initialViewMode = computed(() => {
  const viewFromQuery = route.query.view as string
  if (viewFromQuery && ['standard', 'analytics', 'staff'].includes(viewFromQuery)) {
    return viewFromQuery
  }
  return 'standard'
})
const viewMode = ref(initialViewMode.value)

// Initialize productions subtab from URL query or default to 'all'
const initialProductionsSubTab = computed(() => {
  const subTabFromQuery = route.query.subtab as string
  if (subTabFromQuery && ['all', 'main', 'supporting'].includes(subTabFromQuery)) {
    return subTabFromQuery
  }
  return 'all'
})
const productionsSubTab = ref(initialProductionsSubTab.value)

// Filter state
const selectedGenres = ref<string[]>([])
const selectedTags = ref<string[]>([])
const selectedRatingsRange = ref<[number, number]>([0, 100])

// Card size
const { cardSize, cardColSize, cardsPerRow, showYearMarkers } = useCardSize()

// Sort state
const { sortBy, sortOrder, sortOptions, toggleSortOrder } = useSortable('year', 'desc')

// Filters expansion panel state
const filtersOpen = ref(0)

// Include unrated anime in filter
const includeUnratedAnime = ref(true)

// Display value for rating range slider (only syncs to actual filter on drag end)
const displayRatingsRange = ref<[number, number]>([0, 100])

const onRatingsRangeEnd = () => {
  selectedRatingsRange.value = [...displayRatingsRange.value] as [number, number]
}

// Sync view mode changes to URL
watch(viewMode, (newViewMode) => {
  const query: any = { view: newViewMode }

  // Only include subtab in URL when in standard view
  if (newViewMode === 'standard') {
    query.subtab = productionsSubTab.value
  }

  router.replace({ query })
})

// Sync productions subtab changes to URL (only when in standard view)
watch(productionsSubTab, (newSubTab) => {
  if (viewMode.value === 'standard') {
    router.replace({
      query: {
        view: viewMode.value,
        subtab: newSubTab
      }
    })
  }
})

// Check if any filters are active
const hasActiveFilters = computed(() => {
  const hasRatingFilter = selectedRatingsRange.value[0] > 0 || selectedRatingsRange.value[1] < 100 || !includeUnratedAnime.value
  return selectedGenres.value.length > 0 || selectedTags.value.length > 0 || hasRatingFilter
})

// Get filtered productions based on selected genres/tags/ratings
const getFilteredProductions = (productions: any[]) => {
  if (!hasActiveFilters.value) {
    return productions
  }

  return productions.filter((production: any) => {
    const animeGenres = production.anime?.genres || []
    const animeTags = production.anime?.tags?.map((t: any) => t.name) || []
    const animeScore = production.anime?.averageScore

    const genreMatch = selectedGenres.value.length === 0 ||
      selectedGenres.value.every(g => animeGenres.includes(g))

    const tagMatch = selectedTags.value.length === 0 ||
      selectedTags.value.every(t => animeTags.includes(t))

    // Rating filter: conditionally include anime without ratings based on checkbox
    const hasRatingFilter = selectedRatingsRange.value[0] > 0 || selectedRatingsRange.value[1] < 100 || !includeUnratedAnime.value
    const ratingMatch = !hasRatingFilter ||
      (includeUnratedAnime.value && !animeScore) ||
      (animeScore && animeScore >= selectedRatingsRange.value[0] && animeScore <= selectedRatingsRange.value[1])

    return genreMatch && tagMatch && ratingMatch
  })
}

// Base filtered productions (adult content filter only)
const baseFilteredMainProductions = computed(() => {
  if (!studio.value) return []
  return filterAdultContent(studio.value.mainProductions || [], includeAdult.value)
})

const baseFilteredSupportingProductions = computed(() => {
  if (!studio.value) return []
  return filterAdultContent(studio.value.supportingProductions || [], includeAdult.value)
})

// Sorted productions (automatically reacts to sort changes)
const allProductions = computed(() => {
  const main = baseFilteredMainProductions.value
  const supporting = baseFilteredSupportingProductions.value
  return sortByYearAndScore([...main, ...supporting], sortBy.value, sortOrder.value)
})

const sortedMainProductions = computed(() => {
  return sortByYearAndScore([...baseFilteredMainProductions.value], sortBy.value, sortOrder.value)
})

const sortedSupportingProductions = computed(() => {
  return sortByYearAndScore([...baseFilteredSupportingProductions.value], sortBy.value, sortOrder.value)
})

// Filtered production arrays
const filteredAllProductions = computed(() => getFilteredProductions(allProductions.value))
const filteredMainProductions = computed(() => getFilteredProductions(sortedMainProductions.value))
const filteredSupportingProductions = computed(() => getFilteredProductions(sortedSupportingProductions.value))

// Computed for total and filtered counts
const totalProductionsCount = computed(() => allProductions.value.length)
const filteredProductionsCount = computed(() => filteredAllProductions.value.length)
const totalMainCount = computed(() => sortedMainProductions.value.length)
const filteredMainCount = computed(() => filteredMainProductions.value.length)
const totalSupportingCount = computed(() => sortedSupportingProductions.value.length)
const filteredSupportingCount = computed(() => filteredSupportingProductions.value.length)


// Active base productions (unfiltered by genre/tag) - used for filter count simulation
const activeBaseProductions = computed(() => {
  if (productionsSubTab.value === 'main') return sortedMainProductions.value
  if (productionsSubTab.value === 'supporting') return sortedSupportingProductions.value
  return allProductions.value
})

// Active filtered productions (without year markers)
const activeFilteredProductions = computed(() => {
  if (productionsSubTab.value === 'main') return filteredMainProductions.value
  if (productionsSubTab.value === 'supporting') return filteredSupportingProductions.value
  return filteredAllProductions.value
})

// Pagination
const SLOTS_PER_PAGE = 24
const currentPage = ref(1)

// Slot-aware pagination: year markers and spacers count as slots
const paginatedPages = computed(() => {
  if (sortBy.value !== 'year' || !showYearMarkers.value) {
    return null // use simple item-based pagination
  }
  return paginateWithYearMarkers(activeFilteredProductions.value, cardsPerRow.value, SLOTS_PER_PAGE)
})

const totalPages = computed(() => {
  if (paginatedPages.value) {
    return Math.max(1, paginatedPages.value.length)
  }
  return Math.max(1, Math.ceil(activeFilteredProductions.value.length / SLOTS_PER_PAGE))
})

// Paginated productions (slot-aware when year markers are on)
const activeProductions = computed(() => {
  if (paginatedPages.value) {
    return paginatedPages.value[currentPage.value - 1] || []
  }
  const start = (currentPage.value - 1) * SLOTS_PER_PAGE
  return activeFilteredProductions.value.slice(start, start + SLOTS_PER_PAGE)
})

// Reset page when filters, subtab, or sort changes
watch([productionsSubTab, selectedGenres, selectedTags, selectedRatingsRange, includeUnratedAnime, sortBy, sortOrder], () => {
  currentPage.value = 1
}, { deep: true })

// Active total and filtered counts (for current group)
const activeTotalCount = computed(() => {
  if (productionsSubTab.value === 'main') return totalMainCount.value
  if (productionsSubTab.value === 'supporting') return totalSupportingCount.value
  return totalProductionsCount.value
})

const activeFilteredCount = computed(() => {
  if (productionsSubTab.value === 'main') return filteredMainCount.value
  if (productionsSubTab.value === 'supporting') return filteredSupportingCount.value
  return filteredProductionsCount.value
})

// Genre color mapping
const genreColors = [
  'red', 'pink', 'purple', 'deep-purple', 'indigo', 'blue', 'light-blue', 'cyan',
  'teal', 'green', 'light-green', 'lime', 'yellow', 'amber', 'orange', 'deep-orange',
  'brown', 'blue-grey', 'grey'
]

const genreColorMap = new Map<string, string>()

const getGenreColor = (genre: string): string => {
  if (!genreColorMap.has(genre)) {
    const index = genreColorMap.size % genreColors.length
    genreColorMap.set(genre, genreColors[index])
  }
  return genreColorMap.get(genre)!
}

// Analytics view helpers
const analyticsGenreColors = ['primary', 'secondary', 'accent', 'success', 'warning', 'error', 'info']
const getAnalyticsGenreColor = (index: number) => {
  return analyticsGenreColors[index % analyticsGenreColors.length]
}

const analyticsTagColors = ['secondary', 'accent', 'primary', 'info', 'success', 'warning']
const getAnalyticsTagColor = (index: number) => {
  return analyticsTagColors[index % analyticsTagColors.length]
}

const getGenrePercentage = (count: number) => {
  if (totalProductionsCount.value === 0) return 0
  return Math.round((count / totalProductionsCount.value) * 100)
}

const getTagPercentage = (count: number) => {
  if (totalProductionsCount.value === 0) return 0
  return Math.round((count / totalProductionsCount.value) * 100)
}

// Format distribution analytics
const formatStats = computed(() => {
  if (!studio.value) return []

  const formatCounts = new Map<string, number>()

  allProductions.value.forEach((production: any) => {
    const format = production.anime?.format || 'Unknown'
    formatCounts.set(format, (formatCounts.get(format) || 0) + 1)
  })

  return Array.from(formatCounts.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
})

const getFormatPercentage = (count: number) => {
  if (totalProductionsCount.value === 0) return 0
  return Math.round((count / totalProductionsCount.value) * 100)
}

const analyticsFormatColors = ['info', 'success', 'warning', 'error', 'primary', 'secondary']
const getAnalyticsFormatColor = (index: number) => {
  return analyticsFormatColors[index % analyticsFormatColors.length]
}

// Average score analytics
const overallAverageScore = computed(() => {
  if (!studio.value || allProductions.value.length === 0) return 0

  let totalScore = 0
  let scoreCount = 0

  allProductions.value.forEach((production: any) => {
    if (production.anime?.averageScore) {
      totalScore += production.anime.averageScore
      scoreCount++
    }
  })

  return scoreCount > 0 ? (totalScore / scoreCount) : 0
})

// Score trends by year
const scoreTrendsByYear = computed(() => {
  if (!studio.value) return []

  const yearScores = new Map<number, { total: number; count: number }>()

  allProductions.value.forEach((production: any) => {
    const year = production.anime?.seasonYear
    const score = production.anime?.averageScore

    if (year && score) {
      if (!yearScores.has(year)) {
        yearScores.set(year, { total: 0, count: 0 })
      }
      const yearData = yearScores.get(year)!
      yearData.total += score
      yearData.count += 1
    }
  })

  return Array.from(yearScores.entries())
    .map(([year, data]) => ({
      year,
      averageScore: Math.round(data.total / data.count)
    }))
    .sort((a, b) => a.year - b.year)
})

const minScore = computed(() => {
  if (scoreTrendsByYear.value.length === 0) return 0
  return Math.min(...scoreTrendsByYear.value.map(s => s.averageScore))
})

const maxScore = computed(() => {
  if (scoreTrendsByYear.value.length === 0) return 100
  return Math.max(...scoreTrendsByYear.value.map(s => s.averageScore))
})

// Color coding for scores
const getScoreColor = (score: number) => {
  if (score >= 75) return '#4caf50' // green
  if (score >= 60) return '#8bc34a' // light green
  if (score >= 50) return '#ffc107' // amber
  if (score >= 40) return '#ff9800' // orange
  return '#f44336' // red
}

// Line graph calculations for score trends
const scoreLinePoints = computed(() => {
  if (scoreTrendsByYear.value.length === 0) return []

  const chartWidth = 520 // 580 - 60 (margins)
  const chartHeight = 240 // 270 - 30 (margins)
  const padding = 60
  const bottomPadding = 30

  const points = scoreTrendsByYear.value.map((data, index) => {
    const x = padding + (index / (scoreTrendsByYear.value.length - 1 || 1)) * chartWidth
    // Map score (0-100) to y position (inverted, 0 at top)
    const y = bottomPadding + chartHeight - (data.averageScore / 100) * chartHeight

    return {
      x,
      y,
      year: data.year,
      score: data.averageScore
    }
  })

  return points
})

const scoreLinePath = computed(() => {
  if (scoreLinePoints.value.length === 0) return ''

  const points = scoreLinePoints.value
  if (points.length === 1) {
    // Single point - just draw a dot
    return `M ${points[0].x},${points[0].y}`
  }

  // Create a smooth curve using quadratic bezier curves
  let path = `M ${points[0].x},${points[0].y}`

  for (let i = 0; i < points.length - 1; i++) {
    const current = points[i]
    const next = points[i + 1]

    // Control point for smooth curve
    const midX = (current.x + next.x) / 2

    path += ` Q ${current.x},${current.y} ${midX},${(current.y + next.y) / 2}`
    path += ` Q ${next.x},${next.y} ${next.x},${next.y}`
  }

  return path
})

const scoreAreaPath = computed(() => {
  if (scoreLinePoints.value.length === 0) return ''

  const points = scoreLinePoints.value
  const bottomY = 270

  // Start from bottom left
  let path = `M ${points[0].x},${bottomY}`
  path += ` L ${points[0].x},${points[0].y}`

  // Follow the line path
  for (let i = 0; i < points.length - 1; i++) {
    const current = points[i]
    const next = points[i + 1]
    const midX = (current.x + next.x) / 2

    path += ` Q ${current.x},${current.y} ${midX},${(current.y + next.y) / 2}`
    path += ` Q ${next.x},${next.y} ${next.x},${next.y}`
  }

  // Close the path along the bottom
  path += ` L ${points[points.length - 1].x},${bottomY}`
  path += ' Z'

  return path
})

// Determine which year labels to show to avoid overcrowding
const shouldShowYearLabel = (index: number) => {
  const totalYears = scoreTrendsByYear.value.length

  // Show every year if 8 or fewer
  if (totalYears <= 8) return true

  // Show every other year if 9-16
  if (totalYears <= 16) return index % 2 === 0

  // Show every 3rd year if 17-24
  if (totalYears <= 24) return index % 3 === 0

  // Show every 4th year if more than 24
  return index % 4 === 0
}

const goBack = () => {
  router.back()
}

// Pre-computed filter counts for all three tabs (switching tabs is instant)
const allTabFilterCounts = ref<Record<string, { genres: Record<string, number>; tags: Record<string, number> }>>({
  all: { genres: {}, tags: {} },
  main: { genres: {}, tags: {} },
  supporting: { genres: {}, tags: {} }
})

// Active filter counts based on current tab
const filterCounts = computed(() => allTabFilterCounts.value[productionsSubTab.value] || { genres: {}, tags: {} })

// Function to calculate filter counts for all tabs at once
const calculateFilterCounts = () => {
  if (!studio.value) {
    allTabFilterCounts.value = {
      all: { genres: {}, tags: {} },
      main: { genres: {}, tags: {} },
      supporting: { genres: {}, tags: {} }
    }
    return
  }

  const checkGenres = studio.value.genreStats?.map((g: any) => g.name) || []
  const checkTags = studio.value.tagStats?.map((t: any) => t.name) || []

  if (checkGenres.length === 0 && checkTags.length === 0) {
    allTabFilterCounts.value = {
      all: { genres: {}, tags: {} },
      main: { genres: {}, tags: {} },
      supporting: { genres: {}, tags: {} }
    }
    return
  }

  // Shared filter state
  const hasRatingFilter = selectedRatingsRange.value[0] > 0 || selectedRatingsRange.value[1] < 100 || !includeUnratedAnime.value
  const ratingMin = selectedRatingsRange.value[0]
  const ratingMax = selectedRatingsRange.value[1]
  const includeUnrated = includeUnratedAnime.value
  const curSelectedGenres = selectedGenres.value
  const curSelectedTags = selectedTags.value

  // Pre-extract data for all productions once (all tab is the superset)
  const prepareProductions = (productions: any[]) => {
    return productions.map((production: any) => {
      const genreSet = new Set<string>(production.anime?.genres || [])
      const tagSet = new Set<string>(production.anime?.tags?.map((t: any) => t.name) || [])
      const score = production.anime?.averageScore
      const ratingMatch = !hasRatingFilter ||
        (includeUnrated && !score) ||
        (score && score >= ratingMin && score <= ratingMax)
      return { genreSet, tagSet, ratingMatch }
    })
  }

  const computeCountsForTab = (prepared: ReturnType<typeof prepareProductions>, filteredCount: number) => {
    const counts = { genres: {} as Record<string, number>, tags: {} as Record<string, number> }

    // Pre-check which productions pass current filters
    const passesTagFilter = curSelectedTags.length === 0
      ? null
      : prepared.map(p => curSelectedTags.every(t => p.tagSet.has(t)))

    const passesGenreFilter = curSelectedGenres.length === 0
      ? null
      : prepared.map(p => curSelectedGenres.every(g => p.genreSet.has(g)))

    checkGenres.forEach(genre => {
      if (curSelectedGenres.includes(genre)) {
        counts.genres[genre] = filteredCount
        return
      }
      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.ratingMatch && (passesTagFilter === null || passesTagFilter[i]) && (passesGenreFilter === null || passesGenreFilter[i]) && p.genreSet.has(genre)) {
          count++
        }
      }
      counts.genres[genre] = count
    })

    checkTags.forEach(tag => {
      if (curSelectedTags.includes(tag)) {
        counts.tags[tag] = filteredCount
        return
      }
      let count = 0
      for (let i = 0; i < prepared.length; i++) {
        const p = prepared[i]
        if (p.ratingMatch && (passesGenreFilter === null || passesGenreFilter[i]) && (passesTagFilter === null || passesTagFilter[i]) && p.tagSet.has(tag)) {
          count++
        }
      }
      counts.tags[tag] = count
    })

    return counts
  }

  // Prepare data for each tab
  const preparedAll = prepareProductions(allProductions.value)
  const preparedMain = prepareProductions(sortedMainProductions.value)
  const preparedSupporting = prepareProductions(sortedSupportingProductions.value)

  allTabFilterCounts.value = {
    all: computeCountsForTab(preparedAll, filteredAllProductions.value.length),
    main: computeCountsForTab(preparedMain, filteredMainProductions.value.length),
    supporting: computeCountsForTab(preparedSupporting, filteredSupportingProductions.value.length)
  }
}

// Watch for filter/data changes (NOT productionsSubTab - tab switching is instant via computed)
watch(
  [selectedGenres, selectedTags, selectedRatingsRange, includeUnratedAnime, () => studio.value],
  () => {
    calculateFilterCounts()
  },
  { immediate: true, deep: true }
)

// Toggle genre filter
const toggleGenreFilter = (genre: string) => {
  const index = selectedGenres.value.indexOf(genre)
  if (index === -1) {
    selectedGenres.value = [...selectedGenres.value, genre]
  } else {
    selectedGenres.value = selectedGenres.value.filter(g => g !== genre)
  }
}

// Toggle tag filter
const toggleTagFilter = (tag: string) => {
  const index = selectedTags.value.indexOf(tag)
  if (index === -1) {
    selectedTags.value = [...selectedTags.value, tag]
  } else {
    selectedTags.value = selectedTags.value.filter(t => t !== tag)
  }
}

// Clear all filters
const clearAllFilters = () => {
  selectedGenres.value = []
  selectedTags.value = []
  displayRatingsRange.value = [0, 100]
  selectedRatingsRange.value = [0, 100]
  includeUnratedAnime.value = true
}

// Reset filters and view state when studio changes
watch(studioId, () => {
  selectedGenres.value = []
  selectedTags.value = []
  displayRatingsRange.value = [0, 100]
  selectedRatingsRange.value = [0, 100]
  includeUnratedAnime.value = true
  viewMode.value = initialViewMode.value
  productionsSubTab.value = initialProductionsSubTab.value
})

// Set page title
watchEffect(() => {
  const title = studio.value ? (studio.value.name || 'Unknown Studio') : 'Loading...'
  document.title = title + ' - Anigraph'
})
</script>

<style scoped>
/* Wikipedia content */
.wikipedia-content :deep(a) {
  color: rgb(var(--v-theme-primary));
  text-decoration: none;
}
.wikipedia-content :deep(a:hover) {
  text-decoration: underline;
}
.wikipedia-content :deep(h2),
.wikipedia-content :deep(h3),
.wikipedia-content :deep(h4) {
  margin-top: 1em;
  margin-bottom: 0.5em;
}
.wikipedia-content :deep(ul),
.wikipedia-content :deep(ol) {
  padding-left: 1.5em;
  margin-bottom: 0.75em;
}
.wikipedia-content :deep(p) {
  margin-bottom: 0.75em;
}
.wikipedia-content :deep(sup) {
  display: none;
}
.wikipedia-content.ref-links-only :deep(a) {
  color: inherit;
  text-decoration: none;
  pointer-events: none;
  cursor: text;
}
.wikipedia-content.ref-links-only :deep(a:hover) {
  text-decoration: none;
}
.wikipedia-content.ref-links-only :deep(.references a),
.wikipedia-content.ref-links-only :deep(.reflist a) {
  color: rgb(var(--v-theme-primary));
  pointer-events: auto;
  cursor: pointer;
}
.wikipedia-content.ref-links-only :deep(.references a:hover),
.wikipedia-content.ref-links-only :deep(.reflist a:hover) {
  text-decoration: underline;
}

/* About Short/Full Toggle */
.about-toggle-chip {
  font-weight: 500;
  transition: all 0.2s ease;
}

/* Page Container */
.studio-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
  padding-bottom: 40px;
}

.studio-content {
  animation: fadeIn 0.6s ease-out;
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

/* Loading & Error States */
.loading-container,
.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 80vh;
  color: var(--color-text);
}

/* Sticky Left Card */
.sticky-card {
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
  background: var(--gradient-surface-card);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-medium);
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

/* Studio Image */
.studio-image-container {
  display: flex;
  justify-content: center;
}

.studio-image-container .v-img {
  width: 100%;
  max-width: 200px;
}

/* Quick Stats Row */
.stats-item {
  flex: 1;
  min-width: 0;
}

/* Filter Chips */
.cursor-pointer {
  cursor: pointer;
}

.selected-genre-filter {
  background: var(--gradient-primary) !important;
  color: var(--color-text) !important;
  border-color: transparent !important;
  font-weight: 600;
  box-shadow: 0 0 12px rgba(var(--color-primary-rgb), 0.4);
  margin: 3px !important;
}

.selected-genre-filter:hover {
  transform: translateY(-2px) !important;
  box-shadow: 0 4px 16px var(--color-primary-border-focus) !important;
}

.selected-genre-filter :deep(.v-icon) {
  color: var(--color-text) !important;
}

.selected-tag-filter {
  background: var(--gradient-secondary) !important;
  color: var(--color-text) !important;
  font-weight: 600;
  box-shadow: 0 0 12px rgba(var(--color-secondary-rgb), 0.4);
  margin: 3px !important;
}

.selected-tag-filter:hover {
  transform: translateY(-2px) !important;
  box-shadow: 0 4px 16px rgba(var(--color-secondary-rgb), 0.5) !important;
}

.selected-tag-filter :deep(.v-icon) {
  color: var(--color-text) !important;
}

/* Spacer columns - glass pane effect */
.spacer-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

/* Year Dividers */
.year-marker-col {
  padding-top: 12px !important;
  padding-bottom: 12px !important;
}

/* Standard View */
.standard-section {
  animation: fadeIn 0.8s ease-out;
}

.timeline-select :deep(.v-field) {
  background-color: rgba(var(--color-bg-rgb), 0.8);
  border-color: var(--color-primary-strong);
}

/* Analytics View */
.analytics-section {
  animation: fadeIn 0.8s ease-out;
}

.analytics-card {
  background: linear-gradient(135deg, rgba(var(--color-surface-rgb), 0.7) 0%, rgba(var(--color-bg-rgb), 0.9) 100%);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-medium);
  transition: all var(--transition-base);
  height: 100%;
}

.analytics-card:hover {
  border-color: rgba(var(--color-primary-rgb), 0.4);
  box-shadow: var(--shadow-glow);
  transform: translateY(-2px);
}

.analytics-card-title {
  font-weight: 600;
  font-size: 1.1rem;
  padding: 20px 24px 16px;
}

.analytics-progress-bar {
  box-shadow: 0 2px 8px var(--color-primary-medium);
}

.genre-bar-item,
.tag-bar-item {
  transition: all 0.3s ease;
  padding: 8px;
  border-radius: 8px;
}

.genre-bar-item:hover,
.tag-bar-item:hover {
  transform: translateX(8px);
  background-color: var(--color-primary-faint);
}

.format-bar-item {
  transition: all 0.3s ease;
  padding: 8px;
  border-radius: 8px;
}

.format-bar-item:hover {
  transform: translateX(8px);
  background-color: var(--color-primary-faint);
}

/* Statistics Cards */
.stat-card {
  background: linear-gradient(135deg, rgba(var(--color-surface-rgb), 0.7) 0%, rgba(var(--color-bg-rgb), 0.9) 100%);
  backdrop-filter: blur(10px);
  border: 1px solid var(--color-primary-medium);
  transition: all 0.3s ease;
  height: 100%;
}

.stat-card:hover {
  border-color: rgba(var(--color-primary-rgb), 0.4);
  box-shadow: 0 8px 24px var(--color-primary-medium);
  transform: translateY(-4px);
}

/* Score Trends Line Graph */
.score-trends-chart-container {
  position: relative;
  width: 100%;
  padding: 16px 0;
}

.score-trends-svg {
  width: 100%;
  height: auto;
  overflow: visible;
}

.chart-label {
  fill: rgba(var(--color-text-rgb), 0.7);
  font-size: 12px;
  font-family: system-ui, -apple-system, sans-serif;
}

.chart-label-year {
  font-size: 11px;
  font-weight: 500;
}

.score-line {
  filter: drop-shadow(0 2px 4px var(--color-primary-strong));
  transition: all 0.3s ease;
}

.score-point {
  cursor: pointer;
  transition: all 0.3s ease;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
}

.score-point:hover {
  r: 7;
  filter: drop-shadow(0 3px 6px rgba(0, 0, 0, 0.4));
}

.chart-legend {
  display: flex;
  justify-content: center;
  gap: 16px;
}

.legend-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 6px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.gap-1 {
  gap: 4px;
}

.gap-3 {
  gap: 12px;
}

.h-100 {
  height: 100%;
}

/* Responsive */
@media (max-width: 960px) {
  .studio-page {
    padding-top: 56px;
    padding-bottom: 30px;
  }

  .spacer-col {
    padding-top: 8px !important;
    padding-bottom: 8px !important;
  }

  .year-marker-col {
    padding-top: 8px !important;
    padding-bottom: 8px !important;
  }

  .year-divider {
    padding: 12px 6px;
  }

  .year-label {
    font-size: 1.2rem;
  }

  .year-count {
    font-size: 0.65rem;
  }

  .timeline-header {
    padding: 12px;
  }

  .analytics-card {
    margin-bottom: 16px;
  }
}

@media (max-width: 600px) {
  .spacer-col {
    padding-top: 6px !important;
    padding-bottom: 6px !important;
  }

  .year-marker-col {
    padding-top: 6px !important;
    padding-bottom: 6px !important;
  }

  .year-divider {
    padding: 10px 6px;
  }

  .year-label {
    font-size: 1.1rem;
  }

  .year-count {
    font-size: 0.6rem;
  }

  .timeline-header {
    flex-direction: column;
    align-items: flex-start !important;
    gap: 12px;
  }

  .timeline-select {
    max-width: 100% !important;
    width: 100%;
  }
}
</style>
