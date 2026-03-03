import { ref, shallowRef, computed, watch, nextTick } from "vue";
import type { GraphNode, GraphLink, GraphData } from "@/types/graph";
import {
  STAFF_CATEGORIES,
  categorizeRole,
} from "@/utils/staffCategories";
import { api } from "@/utils/api";

export const useGraphRecommendations = (opts: {
  animeId: () => string;
  graphData: () => GraphData | null;
  filteredData: () => {
    nodes: GraphNode[];
    links: GraphLink[];
    center: string | number;
  } | null;
  filterMetadataLoaded: () => boolean;
  loadingFilterMetadata: () => boolean;
  loadFilterMetadata: () => Promise<void>;
  getFilteredMetadata: (id: number) => any[];
  useBitmaps: () => boolean;
  lookupTables: () => any;
  preGenreTagAnimeNodeIds: () => Set<number>;
  categoryColors: Record<string, string>;
  includeAdult: () => boolean;
  graphAnimeMetadataMap: () => Map<number, any>;
  isInitialLoad: () => boolean;
}) => {
  const ADULT_GENRES = ["Hentai", "Ecchi"];

  // Recommendation mode
  const recommendationMode = ref<"filtered" | "all" | "recommended">(
    "filtered",
  );

  // Pagination
  const currentPage = ref(1);
  const itemsPerPage = ref(12);

  // Recommended anime state (from API)
  const recommendedAnime = shallowRef<
    Array<{
      anilistId: string;
      title: string;
      coverImage?: string;
      similarity: number;
      averageScore?: number | null;
      description?: string | null;
      format?: string | null;
      seasonYear?: number | null;
    }>
  >([]);
  const recommendedLoading = ref(false);
  const recommendedPagination = ref({
    page: 1,
    limit: 12,
    total: 0,
    totalPages: 0,
    hasMore: false,
  });

  // Client-side cache
  const RECOMMENDED_PAGES_PER_FETCH = 3;
  const recommendedCache = shallowRef<any[]>([]);
  const recommendedCacheOffset = ref(-1);

  // Genre/Tag filtering for recommendations
  const recommendationFilters = ref<{ genres: string[]; tags: string[] }>({
    genres: [],
    tags: [],
  });
  const recommendationFilterCounts = ref<any>({ genres: {}, tags: {} });
  const loadingRecommendationCounts = ref(false);

  // Compute recommendations from graph data
  const computeRecommendationsFromData = (
    data: {
      nodes: GraphNode[];
      links: GraphLink[];
      center: string | number;
    } | null,
  ) => {
    if (!data) return [];

    const nodeMap = new Map<string | number, GraphNode>();
    data.nodes.forEach((node) => nodeMap.set(node.id, node));

    const animeStaffCount = new Map<
      string,
      {
        anilistId: string;
        title: string;
        staffCount: number;
        coverImage?: string;
        averageScore?: number | null;
        format?: string | null;
        seasonYear?: number | null;
        categoryBreakdown: Record<string, number>;
      }
    >();

    data.links.forEach((link) => {
      const sourceId =
        typeof link.source === "object" ? link.source.id : link.source;
      const targetId =
        typeof link.target === "object" ? link.target.id : link.target;
      const sourceNode = nodeMap.get(sourceId);
      const targetNode = nodeMap.get(targetId);

      if (sourceNode?.type === "staff" && targetNode?.type === "anime") {
        if (targetNode.id !== data.center) {
          const category = link.category || "other";
          const existing = animeStaffCount.get(targetNode.id as string);
          if (existing) {
            existing.staffCount++;
            existing.categoryBreakdown[category] =
              (existing.categoryBreakdown[category] || 0) + 1;
          } else {
            animeStaffCount.set(targetNode.id as string, {
              anilistId: targetNode.id as string,
              title: targetNode.label,
              staffCount: 1,
              coverImage: targetNode.image,
              averageScore: targetNode.averageScore,
              format: targetNode.format,
              seasonYear: targetNode.seasonYear,
              categoryBreakdown: { [category]: 1 },
            });
          }
        }
      } else if (
        sourceNode?.type === "category" &&
        targetNode?.type === "anime"
      ) {
        if (targetNode.id !== data.center) {
          const category = link.category || "other";
          const staffCount = link.staffCount || 1;
          const existing = animeStaffCount.get(targetNode.id as string);
          if (existing) {
            existing.staffCount += staffCount;
            existing.categoryBreakdown[category] =
              (existing.categoryBreakdown[category] || 0) + staffCount;
          } else {
            animeStaffCount.set(targetNode.id as string, {
              anilistId: targetNode.id as string,
              title: targetNode.label,
              staffCount,
              coverImage: targetNode.image,
              averageScore: targetNode.averageScore,
              format: targetNode.format,
              seasonYear: targetNode.seasonYear,
              categoryBreakdown: { [category]: staffCount },
            });
          }
        }
      } else if (
        link.type === "anime-anime" &&
        sourceNode?.type === "anime" &&
        targetNode?.type === "anime"
      ) {
        if (targetNode.id !== data.center) {
          const category = link.category || "other";
          const staffCount = link.staffCount || 1;
          const existing = animeStaffCount.get(targetNode.id as string);
          if (existing) {
            existing.staffCount += staffCount;
            existing.categoryBreakdown[category] =
              (existing.categoryBreakdown[category] || 0) + staffCount;
          } else {
            animeStaffCount.set(targetNode.id as string, {
              anilistId: targetNode.id as string,
              title: targetNode.label,
              staffCount,
              coverImage: targetNode.image,
              averageScore: targetNode.averageScore,
              format: targetNode.format,
              seasonYear: targetNode.seasonYear,
              categoryBreakdown: { [category]: staffCount },
            });
          }
        }
      }
    });

    return Array.from(animeStaffCount.values()).sort((a, b) => {
      if (b.staffCount !== a.staffCount) return b.staffCount - a.staffCount;
      const scoreA = a.averageScore ?? 0;
      const scoreB = b.averageScore ?? 0;
      if (scoreB !== scoreA) return scoreB - scoreA;
      return a.title.localeCompare(b.title);
    });
  };

  const filteredRecommendations = computed(() =>
    computeRecommendationsFromData(opts.filteredData()),
  );

  const allRecommendations = computed(() =>
    computeRecommendationsFromData(opts.graphData()),
  );

  const recommendations = computed(() => {
    if (recommendationMode.value === "recommended") return recommendedAnime.value;
    return recommendationMode.value === "filtered"
      ? filteredRecommendations.value
      : allRecommendations.value;
  });

  // Available genres/tags from graph anime nodes
  const availableGenres = computed(() => {
    if (
      !opts.filterMetadataLoaded() ||
      opts.preGenreTagAnimeNodeIds().size === 0
    )
      return [];

    const genreSet = new Set<string>();
    opts.preGenreTagAnimeNodeIds().forEach((animeId) => {
      const animeMetadata = opts.graphAnimeMetadataMap().get(animeId);
      if (!animeMetadata) return;

      if (opts.useBitmaps() && opts.lookupTables()) {
        const ids = animeMetadata.g || [];
        ids.forEach((id: number) => {
          const name = opts.lookupTables().genres[id];
          if (name) genreSet.add(name);
        });
      } else {
        const genres = animeMetadata.genres || [];
        genres.forEach((g: string) => genreSet.add(g));
      }
    });

    let genres = Array.from(genreSet).sort();
    if (!opts.includeAdult()) {
      genres = genres.filter((g) => !ADULT_GENRES.includes(g));
    }
    return genres;
  });

  const availableTags = computed(() => {
    if (
      !opts.filterMetadataLoaded() ||
      opts.preGenreTagAnimeNodeIds().size === 0
    )
      return [];

    const tagCounts = new Map<string, number>();
    opts.preGenreTagAnimeNodeIds().forEach((animeId) => {
      const animeMetadata = opts.graphAnimeMetadataMap().get(animeId);
      if (!animeMetadata) return;

      if (opts.useBitmaps() && opts.lookupTables()) {
        const ids = animeMetadata.t || [];
        ids.forEach((id: number) => {
          const name = opts.lookupTables().tags[id];
          if (name) tagCounts.set(name, (tagCounts.get(name) || 0) + 1);
        });
      } else {
        const tags = animeMetadata.tags || [];
        tags.forEach((t: string) => {
          tagCounts.set(t, (tagCounts.get(t) || 0) + 1);
        });
      }
    });

    return Array.from(tagCounts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([name]) => ({ name }));
  });

  // Filtered recommendations by genre/tag
  const filteredRecommendationsByGenreTags = computed(() => {
    const baseRecommendations =
      recommendationMode.value === "filtered"
        ? filteredRecommendations.value
        : recommendationMode.value === "all"
          ? allRecommendations.value
          : [];

    if (
      recommendationFilters.value.genres.length === 0 &&
      recommendationFilters.value.tags.length === 0
    ) {
      return baseRecommendations;
    }

    if (!opts.filterMetadataLoaded()) return baseRecommendations;

    const metadata = opts.getFilteredMetadata(parseInt(opts.animeId()));
    const genreNames = recommendationFilters.value.genres;
    const tagNames = recommendationFilters.value.tags;

    const genreIdsToCheck =
      opts.useBitmaps() && opts.lookupTables()
        ? genreNames
            .map((name: string) => opts.lookupTables().genres.indexOf(name))
            .filter((id: number) => id !== -1)
        : genreNames;

    const tagIdsToCheck =
      opts.useBitmaps() && opts.lookupTables()
        ? tagNames
            .map((name: string) => opts.lookupTables().tags.indexOf(name))
            .filter((id: number) => id !== -1)
        : tagNames;

    const matchingAnimeIds = new Set(
      metadata
        .filter((anime: any) => {
          const genres = opts.useBitmaps() ? anime.g || [] : anime.genres || [];
          const tags = opts.useBitmaps() ? anime.t || [] : anime.tags || [];
          const genreMatch =
            genreIdsToCheck.length === 0 ||
            genreIdsToCheck.every((g: any) => genres.includes(g));
          const tagMatch =
            tagIdsToCheck.length === 0 ||
            tagIdsToCheck.every((t: any) => tags.includes(t));
          return genreMatch && tagMatch;
        })
        .map((anime: any) => anime.id),
    );

    return baseRecommendations.filter((rec: any) =>
      matchingAnimeIds.has(parseInt(rec.anilistId)),
    );
  });

  const paginatedRecommendations = computed(() => {
    if (recommendationMode.value === "recommended") return recommendedAnime.value;
    const start = (currentPage.value - 1) * itemsPerPage.value;
    const end = start + itemsPerPage.value;
    return filteredRecommendationsByGenreTags.value.slice(start, end);
  });

  const totalPages = computed(() => {
    if (recommendationMode.value === "recommended")
      return recommendedPagination.value.totalPages;
    return Math.ceil(recommendations.value.length / itemsPerPage.value);
  });

  const hasRecommendations = computed(() => {
    if (recommendationMode.value === "recommended") {
      return (
        recommendedLoading.value ||
        recommendedAnime.value.length > 0 ||
        recommendedPagination.value.total > 0
      );
    }
    return recommendations.value.length > 0;
  });

  const getStaffByRole = (categoryBreakdown?: Record<string, number>) => {
    if (!categoryBreakdown) return undefined;
    return Object.entries(categoryBreakdown)
      .filter(([_, count]) => count > 0)
      .sort((a, b) => b[1] - a[1])
      .map(([category, count]) => ({
        category,
        count,
        color: opts.categoryColors[category] || opts.categoryColors.other,
      }));
  };

  const getSortedCategories = (categoryBreakdown?: Record<string, number>) => {
    if (!categoryBreakdown) return [];
    return Object.entries(categoryBreakdown)
      .filter(([_, count]) => count > 0)
      .sort((a, b) => b[1] - a[1])
      .map(([category, count]) => ({ category, count }));
  };

  const fetchRecommendedAnime = async (page: number = 1) => {
    const fetchLimit = RECOMMENDED_PAGES_PER_FETCH * itemsPerPage.value;
    const displayStartOffset = (page - 1) * itemsPerPage.value;
    const targetFetchOffset =
      Math.floor(displayStartOffset / fetchLimit) * fetchLimit;

    if (
      recommendedCacheOffset.value === targetFetchOffset &&
      recommendedCache.value.length > 0
    ) {
      const localIdx = displayStartOffset - targetFetchOffset;
      if (localIdx < recommendedCache.value.length) {
        recommendedAnime.value = recommendedCache.value.slice(
          localIdx,
          localIdx + itemsPerPage.value,
        );
        return;
      }
    }

    const apiPage = targetFetchOffset / fetchLimit + 1;
    if (recommendedCache.value.length === 0) {
      recommendedLoading.value = true;
    }
    try {
      const response: any = await api(
        `/anime/${encodeURIComponent(opts.animeId())}/recommendations`,
        { params: { page: String(apiPage), limit: String(fetchLimit) } },
      );

      if (response.success) {
        recommendedCache.value = response.data.recommendations;
        recommendedCacheOffset.value = targetFetchOffset;

        const localIdx = displayStartOffset - targetFetchOffset;
        recommendedAnime.value = recommendedCache.value.slice(
          localIdx,
          localIdx + itemsPerPage.value,
        );

        recommendedPagination.value = {
          page,
          limit: itemsPerPage.value,
          total: response.data.pagination.total,
          totalPages: Math.ceil(
            response.data.pagination.total / itemsPerPage.value,
          ),
          hasMore:
            displayStartOffset + itemsPerPage.value <
            response.data.pagination.total,
        };
      }
    } catch (error: any) {
      recommendedAnime.value = [];
      recommendedPagination.value = {
        page: 1,
        limit: itemsPerPage.value,
        total: 0,
        totalPages: 0,
        hasMore: false,
      };
    } finally {
      recommendedLoading.value = false;
    }
  };

  const fetchRecommendationFilterCounts = async () => {
    if (!opts.filterMetadataLoaded()) {
      if (!opts.loadingFilterMetadata()) {
        await opts.loadFilterMetadata();
      } else {
        return;
      }
    }

    const graphAnimeIds = opts.preGenreTagAnimeNodeIds();
    if (graphAnimeIds.size === 0) return;
    if (!availableGenres.value.length && !availableTags.value.length) return;

    loadingRecommendationCounts.value = true;
    try {
      const checkGenres = availableGenres.value || [];
      const checkTags = availableTags.value.map((t: any) => t.name) || [];
      const metadata = opts.getFilteredMetadata(parseInt(opts.animeId()));
      const graphMetadata = metadata.filter((anime: any) =>
        graphAnimeIds.has(anime.id),
      );

      const genreCounts: { [key: string]: number } = {};
      const tagCounts: { [key: string]: number } = {};
      checkGenres.forEach((g) => (genreCounts[g] = 0));
      checkTags.forEach((t) => (tagCounts[t] = 0));

      const filterGenreIds =
        opts.useBitmaps() && opts.lookupTables()
          ? recommendationFilters.value.genres
              .map((name: string) => opts.lookupTables().genres.indexOf(name))
              .filter((id: number) => id !== -1)
          : recommendationFilters.value.genres;

      const filterTagIds =
        opts.useBitmaps() && opts.lookupTables()
          ? recommendationFilters.value.tags
              .map((name: string) => opts.lookupTables().tags.indexOf(name))
              .filter((id: number) => id !== -1)
          : recommendationFilters.value.tags;

      const checkGenreIds: Map<string, number> | null =
        opts.useBitmaps() && opts.lookupTables()
          ? new Map(
              checkGenres.map((name) => [
                name,
                opts.lookupTables().genres.indexOf(name),
              ]),
            )
          : null;
      const checkTagIds: Map<string, number> | null =
        opts.useBitmaps() && opts.lookupTables()
          ? new Map(
              checkTags.map((name) => [
                name,
                opts.lookupTables().tags.indexOf(name),
              ]),
            )
          : null;

      graphMetadata.forEach((anime: any) => {
        const genres = opts.useBitmaps() ? anime.g || [] : anime.genres || [];
        const tags = opts.useBitmaps() ? anime.t || [] : anime.tags || [];
        const genreMatch =
          filterGenreIds.length === 0 ||
          filterGenreIds.every((g: any) => genres.includes(g));
        const tagMatch =
          filterTagIds.length === 0 ||
          filterTagIds.every((t: any) => tags.includes(t));

        if (genreMatch && tagMatch) {
          checkGenres.forEach((genreName) => {
            const id = checkGenreIds ? checkGenreIds.get(genreName)! : genreName;
            if (genres.includes(id)) genreCounts[genreName]++;
          });
          checkTags.forEach((tagName) => {
            const id = checkTagIds ? checkTagIds.get(tagName)! : tagName;
            if (tags.includes(id)) tagCounts[tagName]++;
          });
        }
      });

      recommendationFilterCounts.value = {
        genres: genreCounts,
        tags: tagCounts,
      };
    } catch (error) {
      console.error("Error computing filter counts:", error);
    } finally {
      loadingRecommendationCounts.value = false;
    }
  };

  const addGenreFilter = (genre: string) => {
    if (!recommendationFilters.value.genres.includes(genre)) {
      recommendationFilters.value.genres.push(genre);
    }
  };

  const removeGenreFilter = (genre: string) => {
    recommendationFilters.value.genres =
      recommendationFilters.value.genres.filter((g) => g !== genre);
  };

  const addTagFilter = (tag: string) => {
    if (!recommendationFilters.value.tags.includes(tag)) {
      recommendationFilters.value.tags.push(tag);
    }
  };

  const removeTagFilter = (tag: string) => {
    recommendationFilters.value.tags = recommendationFilters.value.tags.filter(
      (t) => t !== tag,
    );
  };

  const clearRecommendationFilters = () => {
    recommendationFilters.value.genres = [];
    recommendationFilters.value.tags = [];
  };

  const hasRecommendationFilters = computed(() => {
    return (
      recommendationFilters.value.genres.length > 0 ||
      recommendationFilters.value.tags.length > 0
    );
  });

  // Internal watchers
  watch(recommendations, () => {
    if (recommendationMode.value !== "recommended") {
      currentPage.value = 1;
    }
  });

  watch(recommendationMode, (newMode) => {
    currentPage.value = 1;
    if (newMode === "recommended") {
      recommendedCache.value = [];
      recommendedCacheOffset.value = -1;
      fetchRecommendedAnime(1);
    }
    if (newMode !== "recommended") {
      nextTick(() => {
        fetchRecommendationFilterCounts();
      });
    }
  });

  watch(currentPage, (newPage) => {
    if (recommendationMode.value === "recommended") {
      fetchRecommendedAnime(newPage);
    }
  });

  // Reset on anime change
  const resetState = () => {
    recommendedAnime.value = [];
    recommendedCache.value = [];
    recommendedCacheOffset.value = -1;
    recommendedPagination.value = {
      page: 1,
      limit: 12,
      total: 0,
      totalPages: 0,
      hasMore: false,
    };
    recommendationFilters.value = { genres: [], tags: [] };
    currentPage.value = 1;
  };

  return {
    // State
    recommendationMode,
    currentPage,
    itemsPerPage,
    recommendedAnime,
    recommendedLoading,
    recommendedPagination,
    recommendationFilters,
    recommendationFilterCounts,
    loadingRecommendationCounts,

    // Computed
    filteredRecommendations,
    allRecommendations,
    recommendations,
    filteredRecommendationsByGenreTags,
    paginatedRecommendations,
    totalPages,
    hasRecommendations,
    hasRecommendationFilters,
    availableGenres,
    availableTags,

    // Methods
    computeRecommendationsFromData,
    getStaffByRole,
    getSortedCategories,
    fetchRecommendedAnime,
    fetchRecommendationFilterCounts,
    addGenreFilter,
    removeGenreFilter,
    addTagFilter,
    removeTagFilter,
    clearRecommendationFilters,
    resetState,
  };
};
