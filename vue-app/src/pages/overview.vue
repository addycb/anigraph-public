<template>
  <v-app>
    <AppBar clickable-title @title-click="scrollToTop" />

    <v-main class="overview-page">
      <!-- Hero Section with Search -->
      <v-container fluid class="hero-section">
        <div class="hero-content">
          <div class="hero-text">
            <h1 class="hero-title">Discover Anime Connections</h1>
            <p class="hero-subtitle">
              Visualize staff connections, find similar works by shared
              creators, and understand what connects anime beyond genre
            </p>
          </div>

          <div class="search-container">
            <SearchBar
              floating
              show-arrow-button
              density="comfortable"
              hide-details
              label=""
              placeholder="Search works, staff, studios..."
              tracking-source="overview"
              @search="handleSearch"
            />
          </div>
        </div>
      </v-container>

      <!-- Featured Connections: "From the team behind..." (graph-powered) -->
      <template v-for="feat in featuredConnections" :key="feat.animeId">
        <SectionCarousel
          v-if="feat.loading || feat.items.length > 0"
          :title="`From the team behind ${feat.title}`"
          :subtitle="
            feat.staffTotal
              ? `${feat.staffTotal} staff documented — explore which anime share the most creative DNA`
              : 'Anime connected through shared creative staff'
          "
          :items="feat.items"
          :loading="feat.loading"
          icon="mdi-graph"
          color="#60A5FA"
          staff-count-label="shared"
          :view-all-href="`/anime/${feat.animeId}`"
          @view-all="router.push(`/anime/${feat.animeId}`)"
        />
      </template>

      <!-- Filtered carousel: movies from the Madoka Magica team -->
      <SectionCarousel
        v-if="filteredCarousel.loading || filteredCarousel.items.length > 0"
        :title="filteredCarousel.carouselTitle"
        :subtitle="filteredCarousel.subtitle"
        :items="filteredCarousel.items"
        :loading="filteredCarousel.loading"
        icon="mdi-movie-filter"
        color="#F472B6"
        staff-count-label="shared"
        :view-all-href="filteredCarouselHref"
        @view-all="navigateToFilteredCarousel()"
      />

      <!-- Most Documented Works -->
      <SectionCarousel
        ref="mostDocumentedSection"
        title="Most Documented Works"
        subtitle="Ranked by staff connections in our database — see the depth of what we track"
        :items="mostConnected"
        :loading="loadingMostConnected"
        icon="mdi-database-search"
        color="#A78BFA"
        staff-count-label="staff"
        :show-staff-count="true"
        :view-all-href="mostStaffHref"
        @view-all="navigateToFiltered('most-staff')"
      />

      <!-- Genre Explorer Section -->
      <v-container fluid class="genre-section" ref="genreSection">
        <div class="section-header">
          <div class="header-left">
            <v-icon size="28" color="primary">mdi-tag-multiple</v-icon>
            <h2 class="section-title">Explore by Genre</h2>
          </div>
        </div>

        <div v-if="loadingGenres" class="genre-loading">
          <v-progress-circular
            indeterminate
            color="primary"
            size="48"
          ></v-progress-circular>
        </div>

        <div v-else class="genre-grid">
          <v-chip
            v-for="genre in popularGenres"
            :key="genre"
            size="large"
            class="genre-chip"
            @click="navigateToGenre(genre)"
          >
            {{ genre }}
          </v-chip>
        </div>

        <div class="view-all-genres">
          <v-btn
            variant="text"
            color="primary"
            @click="navigateToAdvancedSearch"
            @auxclick.prevent="
              $event.button === 1 && openInNewTab('/search/advanced')
            "
          >
            View All Genres & Tags
            <v-icon end>mdi-arrow-right</v-icon>
          </v-btn>
        </div>
      </v-container>

      <!-- Quick Actions -->
      <v-container fluid class="quick-actions-section">
        <h2 class="section-title text-center mb-8">Quick Actions</h2>

        <v-row justify="center">
          <v-col cols="12" sm="6" md="3">
            <v-card
              class="action-card"
              hover
              @click="router.push('/search/advanced')"
            >
              <v-card-text class="text-center">
                <v-icon size="48" color="primary" class="mb-4"
                  >mdi-filter-variant</v-icon
                >
                <h3 class="action-title">Advanced Search</h3>
                <p class="action-description">
                  Find anime with powerful filters
                </p>
              </v-card-text>
            </v-card>
          </v-col>

          <v-col cols="12" sm="6" md="3">
            <v-card class="action-card" hover @click="router.push('/tutorial')">
              <v-card-text class="text-center">
                <v-icon size="48" color="primary" class="mb-4"
                  >mdi-school</v-icon
                >
                <h3 class="action-title">Take the Tour</h3>
                <p class="action-description">Learn how Anigraph works</p>
              </v-card-text>
            </v-card>
          </v-col>

          <v-col cols="12" sm="6" md="3">
            <v-card class="action-card" hover @click="getRandomAnime">
              <v-card-text class="text-center">
                <v-icon size="48" color="primary" class="mb-4"
                  >mdi-dice-multiple</v-icon
                >
                <h3 class="action-title">Feeling Lucky?</h3>
                <p class="action-description">Discover something random</p>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>
      </v-container>
    </v-main>

    <AppFooter />
  </v-app>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useSettings } from '@/composables/useSettings'
import { useAuth } from '@/composables/useAuth'
import { useFilterMetadata } from '@/composables/useFilterMetadata'
import { api } from '@/utils/api'

interface AnimeData {
  id?: string;
  anilistId?: number;
  title?: string;
  titleEnglish?: string;
  titleRomaji?: string;
  coverImage?: string;
  coverImage_large?: string;
  coverImage_extraLarge?: string;
  averageScore?: number;
  season?: string;
  seasonYear?: number;
  format?: string;
  staffCount?: number;
  description?: string;
}

interface FeaturedConnection {
  animeId: number;
  title: string;
  staffTotal: number;
  items: AnimeData[];
  loading: boolean;
}

interface FilteredCarousel {
  animeId: number;
  carouselTitle: string;
  subtitle: string;
  genreFilter: string[];
  formatFilter?: string[];
  items: AnimeData[];
  loading: boolean;
}

const router = useRouter();
const { includeAdult } = useSettings();
const { isAuthenticated } = useAuth();
const {
  filterMetadataLoaded,
  loadFilterMetadata,
  getFilteredMetadata,
  useBitmaps,
  lookupTables,
} = useFilterMetadata();

// "From the team behind..." graph-powered connections
const FEATURED_ANIME = [{ animeId: 1, title: "Cowboy Bebop" }] as const;

const featuredConnections = ref<FeaturedConnection[]>(
  FEATURED_ANIME.map((a) => ({
    ...a,
    staffTotal: 0,
    items: [],
    loading: true,
  })),
);

// Filtered carousel: Mahou Shoujo from the Madoka Magica team (genre filtered client-side)
const filteredCarousel = ref<FilteredCarousel>({
  animeId: 9756, // Puella Magi Madoka Magica
  carouselTitle: "Fantasy Anime from the Madoka Team",
  subtitle:
    "Fantasy TV anime made by the creative staff behind Puella Magi Madoka Magica",
  genreFilter: ["Fantasy"],
  formatFilter: ["TV"],
  items: [],
  loading: true,
});

// Most documented works (ranked by staff count in DB)
const mostConnected = ref<AnimeData[]>([]);
const loadingMostConnected = ref(true);

// -- Graph helpers --

const computeFromGraph = (
  graphData: any,
  options?: { formats?: string[] },
): { items: AnimeData[]; staffTotal: number } => {
  if (!graphData?.nodes || !graphData?.links)
    return { items: [], staffTotal: 0 };

  const nodeMap = new Map<string | number, any>();
  graphData.nodes.forEach((node: any) => nodeMap.set(node.id, node));

  const centerStaffIds = new Set<string | number>();
  const animeStaffCount = new Map<string, any>();

  graphData.links.forEach((link: any) => {
    const sourceId =
      typeof link.source === "object" ? link.source.id : link.source;
    const targetId =
      typeof link.target === "object" ? link.target.id : link.target;
    const sourceNode = nodeMap.get(sourceId);
    const targetNode = nodeMap.get(targetId);

    // Center -> staff
    if (
      sourceNode?.type === "anime" &&
      sourceNode.id === graphData.center &&
      targetNode?.type === "staff"
    ) {
      centerStaffIds.add(targetId);
    }
    // Staff -> related anime (format filter applied here, same as graph node visibility)
    if (
      sourceNode?.type === "staff" &&
      targetNode?.type === "anime" &&
      String(targetNode.id) !== String(graphData.center)
    ) {
      if (
        options?.formats?.length &&
        !options.formats.includes(targetNode.format)
      )
        return;
      const key = String(targetNode.id);
      const existing = animeStaffCount.get(key);
      if (existing) {
        existing.staffCount++;
      } else {
        animeStaffCount.set(key, {
          id: targetNode.id,
          anilistId: targetNode.id,
          title: targetNode.label,
          coverImage: targetNode.image,
          averageScore: targetNode.averageScore,
          format: targetNode.format,
          season: targetNode.season,
          seasonYear: targetNode.seasonYear,
          staffCount: 1,
        });
      }
    }
  });

  const items = Array.from(animeStaffCount.values())
    .sort((a, b) => b.staffCount - a.staffCount)
    .slice(0, 50)
    .map(({ staffCount: _, ...item }) => item);

  return { items, staffTotal: centerStaffIds.size };
};

const filterByGenre = (items: AnimeData[], genres: string[]): AnimeData[] => {
  if (!genres.length || !filterMetadataLoaded.value) return items;

  const metadataMap = new Map<number, any>(
    getFilteredMetadata().map((a: any) => [a.id, a]),
  );

  return items.filter((item) => {
    if (!item.anilistId) return false;
    const meta = metadataMap.get(item.anilistId);
    if (!meta) return false;

    const animeGenres: any[] = useBitmaps.value
      ? meta.g || []
      : meta.genres || [];

    return genres.every((genreName) => {
      if (useBitmaps.value && lookupTables.value) {
        const genreId = lookupTables.value.genres.indexOf(genreName);
        return genreId !== -1 && animeGenres.includes(genreId);
      }
      return animeGenres.includes(genreName);
    });
  });
};

const enrichWithDescriptions = async (
  items: AnimeData[],
): Promise<AnimeData[]> => {
  if (!items.length) return items;
  const ids = items
    .map((i) => i.anilistId)
    .filter(Boolean)
    .join(",");
  try {
    const res = (await api("/anime/bulk", { params: { ids } })) as any;
    if (res?.success && res.data?.length) {
      const descMap = new Map<number, string | null>(
        res.data.map((d: any) => [d.id, d.description]),
      );
      return items.map((item) => ({
        ...item,
        description: descMap.get(item.anilistId!) ?? item.description,
      }));
    }
  } catch {
    /* non-critical */
  }
  return items;
};

// -- Data loading --

const loadFeaturedConnections = async () => {
  await Promise.all(
    FEATURED_ANIME.map(async (anime, idx) => {
      try {
        const response = (await api(`/graph/${anime.animeId}`)) as any;
        if (response?.success && response.data) {
          const { items, staffTotal } = computeFromGraph(response.data);
          featuredConnections.value[idx].items = await enrichWithDescriptions(
            items.slice(0, 12),
          );
          featuredConnections.value[idx].staffTotal = staffTotal;
        }
      } catch {
        // graph not in cache yet -- section hides itself via v-if
      } finally {
        featuredConnections.value[idx].loading = false;
      }
    }),
  );
};

const loadFilteredCarousel = async () => {
  try {
    // Ensure filter metadata is available for genre filtering
    if (!filterMetadataLoaded.value) await loadFilterMetadata();

    const response = (await api(
      `/graph/${filteredCarousel.value.animeId}`,
    )) as any;
    if (response?.success && response.data) {
      const { items } = computeFromGraph(response.data, {
        formats: filteredCarousel.value.formatFilter,
      });
      const genreFiltered = filterByGenre(
        items,
        filteredCarousel.value.genreFilter,
      ).slice(0, 12);
      filteredCarousel.value.items =
        await enrichWithDescriptions(genreFiltered);
    }
  } catch {
    // not cached -- hide via v-if
  } finally {
    filteredCarousel.value.loading = false;
  }
};

const loadingGenres = ref(true);
const popularGenres = ref<string[]>([]);

const mostDocumentedSection = ref<HTMLElement | null>(null);
const genreSection = ref<HTMLElement | null>(null);

const fetchAnime = async (
  sort: string,
  limit: number = 10,
  additionalParams: Record<string, any> = {},
): Promise<AnimeData[]> => {
  try {
    const params: Record<string, any> = {
      sort,
      limit,
      includeAdult: includeAdult.value,
      ...additionalParams,
    };
    const response = await api("/anime/popular", { params });
    return response.success && response.data ? response.data : [];
  } catch (error) {
    console.error(`Error fetching ${sort} anime:`, error);
    return [];
  }
};

const fetchGenres = async (): Promise<void> => {
  try {
    const response = await api("/anime/genres-tags", {
      params: { includeAdult: includeAdult.value },
    });
    if (response.success) {
      popularGenres.value = response.genres?.slice(0, 20) || [];
    }
  } catch (error) {
    console.error("Error fetching genres:", error);
  } finally {
    loadingGenres.value = false;
  }
};

// -- Navigation --

const handleSearch = (query: string): void => {
  router.push({
    path: "/home",
    query: { q: query, includeAdult: includeAdult.value ? "true" : undefined },
  });
};

const navigateToFiltered = (sort: string): void => {
  router.push({
    path: "/home",
    query: {
      sort,
      type: "anime",
      includeAdult: includeAdult.value ? "true" : undefined,
    },
  });
};

const navigateToGenre = (genre: string): void => {
  router.push({
    path: "/home",
    query: {
      genres: genre,
      includeAdult: includeAdult.value ? "true" : undefined,
    },
  });
};

const navigateToAdvancedSearch = (): void => router.push("/search/advanced");
const openInNewTab = (path: string): void => {
  window.open(path, "_blank");
};

const mostStaffHref = computed(() => {
  const params = new URLSearchParams({ sort: "most-staff", type: "anime" });
  if (includeAdult.value) params.set("includeAdult", "true");
  return `/home?${params.toString()}`;
});

const filteredCarouselHref = computed(() => {
  const query = new URLSearchParams();
  if (filteredCarousel.value.formatFilter?.length)
    query.set("g_formats", filteredCarousel.value.formatFilter.join(","));
  if (filteredCarousel.value.genreFilter.length)
    query.set("g_genres", filteredCarousel.value.genreFilter.join(","));
  const qs = query.toString();
  return `/anime/${filteredCarousel.value.animeId}${qs ? "?" + qs : ""}`;
});

const navigateToFilteredCarousel = (): void => {
  const query: Record<string, string> = {};
  if (filteredCarousel.value.formatFilter?.length) {
    query.g_formats = filteredCarousel.value.formatFilter.join(",");
  }
  if (filteredCarousel.value.genreFilter.length) {
    query.g_genres = filteredCarousel.value.genreFilter.join(",");
  }
  router.push({ path: `/anime/${filteredCarousel.value.animeId}`, query });
};

const getRandomAnime = async (): Promise<void> => {
  const randomAnime = await fetchAnime("random", 1);
  if (randomAnime.length > 0) {
    const animeId = randomAnime[0].id || randomAnime[0].anilistId;
    router.push(`/anime/${animeId}`);
  }
};

const scrollToTop = (): void => window.scrollTo({ top: 0, behavior: "smooth" });

// -- Lifecycle --

let observer: IntersectionObserver | null = null;

onBeforeUnmount(() => observer?.disconnect());

onMounted(async () => {
  document.title = 'Anigraph - Discover Anime Connections'

  // All graph fetches + genres in parallel
  await Promise.all([
    loadFeaturedConnections(),
    loadFilteredCarousel(),
    fetchGenres(),
  ]);

  // Lazy-load the most documented carousel on scroll
  observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const target = entry.target;
          if (
            (target === (mostDocumentedSection.value as any)?.$el ||
              target === mostDocumentedSection.value) &&
            loadingMostConnected.value &&
            mostConnected.value.length === 0
          ) {
            fetchAnime("most-staff", 12, {
              type: "anime",
              minStaff: 10,
              hasRating: true,
            }).then((data) => {
              mostConnected.value = data;
              loadingMostConnected.value = false;
            });
          }
        }
      });
    },
    { root: null, rootMargin: "100px", threshold: 0.1 },
  );

  await nextTick();
  if (mostDocumentedSection.value) {
    observer.observe(
      (mostDocumentedSection.value as any).$el || mostDocumentedSection.value,
    );
  }
});
</script>

<style scoped>
.overview-page {
  background-color: var(--color-bg);
  min-height: 100vh;
  padding-top: 64px;
}

/* Hero Section */
.hero-section {
  background: transparent;
  padding: 72px 24px 48px;
  margin-bottom: 24px;
  border-bottom: 1px solid var(--color-primary-border);
}

.hero-content {
  max-width: 720px;
  margin: 0 auto;
  text-align: center;
}

.hero-text {
  margin-bottom: 36px;
}

.hero-title {
  font-size: 2.75rem;
  font-weight: 700;
  color: var(--color-text);
  margin-bottom: 14px;
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.hero-subtitle {
  font-size: 1.1rem;
  color: rgba(var(--color-text-rgb), 0.65);
  max-width: 560px;
  margin: 0 auto;
  line-height: 1.6;
  font-weight: 400;
}

.search-container {
  max-width: 520px;
  margin: 0 auto;
}

.search-container :deep(.floating-search-input) {
  position: static !important;
  transform: none !important;
  width: 100% !important;
  max-width: none !important;
}

.search-container :deep(.v-field) {
  box-shadow: var(--shadow-sm) !important;
  border: 1px solid var(--color-primary-border) !important;
}

.search-container :deep(.v-field:hover) {
  border-color: var(--color-primary-border-focus) !important;
}

.search-container :deep(.v-field--focused) {
  border-color: var(--color-primary) !important;
  box-shadow: var(--shadow-glow) !important;
}

/* Section Headers */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.section-title {
  font-size: 2rem;
  font-weight: 600;
  color: var(--color-text);
  margin: 0;
  letter-spacing: -0.01em;
}

/* Genre Section */
.genre-section {
  padding: 48px 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.genre-loading {
  display: flex;
  justify-content: center;
  padding: 48px;
}

.genre-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: center;
  margin-bottom: 32px;
}

.genre-chip {
  font-size: 1rem;
  font-weight: 500;
  padding: 8px 16px;
  cursor: pointer;
  transition: all 0.3s ease;
  background: var(--color-primary-muted) !important;
  border: 1px solid var(--color-primary-strong);
}

.genre-chip:hover {
  background: var(--color-primary-strong) !important;
  transform: translateY(-2px);
  box-shadow: var(--shadow-glow);
}

.view-all-genres {
  text-align: center;
}

/* Quick Actions Section */
.quick-actions-section {
  padding: 64px 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.action-card {
  background: rgba(var(--color-surface-rgb), 0.6);
  border: 1px solid var(--color-primary-border);
  transition: all 0.3s ease;
  cursor: pointer;
  height: 100%;
}

.action-card:hover {
  background: rgba(var(--color-surface-rgb), 0.8);
  border-color: var(--color-primary-border-focus);
  transform: translateY(-6px);
  box-shadow: var(--shadow-glow);
}

.action-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 8px;
}

.action-description {
  font-size: 0.875rem;
  color: rgba(var(--color-text-rgb), 0.7);
  margin: 0;
}

/* Responsive Design */
@media (max-width: 960px) {
  .hero-section {
    padding: 56px 24px 40px;
  }

  .hero-title {
    font-size: 2.25rem;
  }

  .hero-subtitle {
    font-size: 1.05rem;
  }

  .section-title {
    font-size: 1.75rem;
  }
}

@media (max-width: 600px) {
  .overview-page {
    padding-top: 56px;
  }

  .hero-section {
    padding: 40px 16px 32px;
    margin-bottom: 16px;
  }

  .hero-title {
    font-size: 1.75rem;
  }

  .hero-subtitle {
    font-size: 0.95rem;
  }

  .section-title {
    font-size: 1.5rem;
  }

  .genre-section,
  .quick-actions-section {
    padding: 32px 16px;
  }

  .genre-chip {
    font-size: 0.875rem;
  }

  .action-title {
    font-size: 1.125rem;
  }
}
</style>
