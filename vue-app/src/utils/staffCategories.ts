export interface StaffCategory {
  key: string
  title_en: string
  title_ja: string
  roles: string[]
  order: number
}

export interface CategoryGroup {
  key: string
  title_en: string
  title_ja: string
  order: number
  children: string[]  // keys of child STAFF_CATEGORIES
}

// 17 detailed staff categories
export const STAFF_CATEGORIES: StaffCategory[] = [
  {
    key: 'series_direction',
    title_en: 'Series Direction',
    title_ja: 'シリーズ監督',
    order: 1,
    roles: [
      'Director',
      '監督',
      'Chief Director',
      '総監督',
      'Series Director',
      'シリーズディレクター',
      'Assistant Director',
      '副監督',
      '助監督',
      'Series Episode Director',
      'シリーズ演出',
      'Chief Episode Director',
      'チーフ演出',
    ]
  },
  {
    key: 'episode_direction',
    title_en: 'Episode Direction & Storyboard',
    title_ja: '演出・絵コンテ',
    order: 2,
    roles: [
      'Episode Director',
      '演出',
      'Storyboard',
      '絵コンテ',
      'Unit Director',
      'ユニットディレクター',
      'Assistant Episode Director',
      '演出補',
      'Storyboard Assistant',
      '絵コンテ補',
    ]
  },
  {
    key: 'writing',
    title_en: 'Writing & Story',
    title_ja: '脚本・原作',
    order: 3,
    roles: [
      'Original Work',
      '原作',
      'Original Story',
      '原案',
      'Series Composition',
      'シリーズ構成',
      'Script',
      '脚本',
      'Screenplay',
      'Scenario',
      'シナリオ',
      'Story',
      'ストーリー',
    ]
  },
  {
    key: 'character_design',
    title_en: 'Character Design',
    title_ja: 'キャラクターデザイン',
    order: 4,
    roles: [
      'Character Design',
      'キャラクターデザイン',
      'Original Character Design',
      '原案',
      'Costume Design',
      'コスチュームデザイン',
      'Character Designer',
      'キャラデザ',
      'Assistant Character Designer',
      'キャラクターデザイン協力',
      'キャラクターデザイン補佐',
      'Sub-Character Design',
      'サブキャラクターデザイン',
    ]
  },
  {
    key: 'visual_design',
    title_en: 'Visual Design',
    title_ja: 'ビジュアルデザイン',
    order: 5,
    roles: [
      'Prop Design',
      'プロップデザイン',
      'Mechanical Design',
      'メカニックデザイン',
      'メカデザイン',
      'Mecha Design',
      'Creature Design',
      'クリーチャーデザイン',
      'Monster Design',
      'モンスターデザイン',
      'Conceptual Design',
      'コンセプトデザイン',
      'World Design',
      '世界観設定',
      'Title Logo Design',
      'タイトルロゴデザイン',
      'ロゴデザイン',
      'Typography Design',
      'フォントデザイン',
      'フォント協力',
    ]
  },
  {
    key: 'music',
    title_en: 'Music & Composition',
    title_ja: '音楽',
    order: 6,
    roles: [
      'Music',
      '音楽',
      'Composer',
      '作曲',
      'Music Composition',
      '音楽制作',
      'Theme Song',
      '主題歌',
      'Opening Theme',
      'OP',
      'Ending Theme',
      'ED',
      'Lyricist',
      '作詞',
      'Arranger',
      '編曲',
    ]
  },
  {
    key: 'opening_ending',
    title_en: 'Opening & Ending',
    title_ja: 'OP・ED',
    order: 7,
    roles: [
      'OP Director',
      'OP演出',
      'OP Storyboard',
      'OP絵コンテ',
      'OP Animation',
      'OP作画',
      'ED Director',
      'ED演出',
      'ED Storyboard',
      'ED絵コンテ',
      'ED Animation',
      'ED作画',
      'Opening Animation Director',
      'OP作画監督',
      'Ending Animation Director',
      'ED作画監督',
    ]
  },
  {
    key: 'animation_supervision',
    title_en: 'Animation Supervision',
    title_ja: '作画監督',
    order: 8,
    roles: [
      'Chief Animation Director',
      '総作画監督',
      '総作監',
      'Animation Director',
      '作画監督',
      '作監',
      'Assistant Animation Director',
      '作画監督補佐',
      'Action Animation Director',
      'アクション作画監督',
      'アクション作監',
      'Mechanical Animation Director',
      'メカ作画監督',
      'メカ作監',
      'Mecha Animation Director',
      'Effects Animation Director',
      'エフェクト作画監督',
      'エフェクト作監',
      'Layout Animation Director',
      'レイアウト作画監督',
      'Chief Chief Animation Director',
      '総総作画監督',
    ]
  },
  {
    key: 'key_animation',
    title_en: 'Key Animation & Layout',
    title_ja: '原画・レイアウト',
    order: 9,
    roles: [
      'Key Animation',
      '原画',
      '一原',
      'Key Animator',
      'キーアニメーター',
      '2nd Key Animation',
      '第二原画',
      'Layout',
      'レイアウト',
      'Genga',
    ]
  },
  {
    key: 'inbetween_animation',
    title_en: 'In-Between Animation',
    title_ja: '動画',
    order: 10,
    roles: [
      'In-Between Animation',
      '動画',
      'In-Between Check',
      '動画検査',
      'Douga',
    ]
  },
  {
    key: 'color_design',
    title_en: 'Color Design & Finishing',
    title_ja: '色彩設計・仕上',
    order: 11,
    roles: [
      'Color Design',
      '色彩設計',
      '色彩設定',
      'Color Designer',
      'Color Coordination',
      '色指定',
      'Color Coordinator',
      'Finishing',
      '仕上',
      '仕上げ',
      'Paint',
      'ペイント',
      'Digital Paint',
      'デジタルペイント',
      'Digital Coloring',
      'デジタル彩色',
      'Finishing Check',
      '仕上検査',
      '仕上げ検査',
    ]
  },
  {
    key: 'art_direction',
    title_en: 'Art Direction & Backgrounds',
    title_ja: '美術',
    order: 12,
    roles: [
      'Art Director',
      '美術監督',
      'Art Designer',
      '美術デザイン',
      'Background Art',
      '背景',
      'Art Design',
      '美術設定',
      'Setting',
      '設定',
      'Art Setting',
    ]
  },
  {
    key: 'photography',
    title_en: 'Photography & Compositing',
    title_ja: '撮影・合成',
    order: 13,
    roles: [
      'Director of Photography',
      '撮影監督',
      'Photography',
      '撮影',
      'Composite',
      '合成',
      'Compositing',
      'Special Effects',
      '特殊効果',
      '2D Works',
      '2Dワークス',
      '2Dデザイン',
      'Monitor Graphics',
      'モニターグラフィックス',
    ]
  },
  {
    key: 'cg',
    title_en: '3DCG',
    title_ja: '3DCG',
    order: 14,
    roles: [
      '3DCG Director',
      '3DCG監督',
      '3DCG',
      '3D Animation',
      '3Dアニメーション',
      'CG Modeling',
      'CGモデリング',
      '3D Modeling',
      '3Dモデリング',
      'CG Animation',
      'CGアニメーション',
      'Rigging',
      'リギング',
    ]
  },
  {
    key: 'editing',
    title_en: 'Editing',
    title_ja: '編集',
    order: 15,
    roles: [
      'Editing',
      '編集',
      'Editor',
      'Developing',
      '現像',
    ]
  },
  {
    key: 'sound_production',
    title_en: 'Sound Production',
    title_ja: '音響',
    order: 16,
    roles: [
      'Sound Director',
      '音響監督',
      'Sound Production',
      '音響制作',
      'Sound Production Manager',
      '音響制作担当',
      'Sound Effects',
      '効果',
      'Recording',
      '録音',
      'Recording Studio',
      '録音スタジオ',
      'Audio Director',
      '音響ディレクター',
    ]
  },
  {
    key: 'production',
    title_en: 'Production & Planning',
    title_ja: '制作',
    order: 17,
    roles: [
      'Producer',
      'プロデューサー',
      'Executive Producer',
      'エグゼクティブプロデューサー',
      'Line Producer',
      'ラインプロデューサー',
      'Production',
      '製作',
      'Production Producer',
      '制作プロデューサー',
      'Production Desk',
      '制作デスク',
      'Production Manager',
      '制作担当',
      '制作マネージャー',
      'Production Assistant',
      '制作進行',
      'Design Production',
      '設定制作',
      'Design Manager',
      '設定マネージャー',
      'Design Control',
      '設定管理',
      'General Planning',
      '総合企画',
      'Planning',
      '企画',
      'Animation Production',
      '制作',
      'Production Cooperation',
      '制作協力',
      'Animation Production Cooperation',
      'アニメーション制作協力',
      'Production Committee / Copyright',
      '製作・著作',
    ]
  },
]

// 9 parent groups that group the 17 detailed categories
export const CATEGORY_GROUPS: CategoryGroup[] = [
  {
    key: 'direction',
    title_en: 'Direction',
    title_ja: '監督',
    order: 1,
    children: ['series_direction', 'episode_direction']
  },
  {
    key: 'writing_story',
    title_en: 'Writing & Story',
    title_ja: '脚本・原作',
    order: 2,
    children: ['writing']
  },
  {
    key: 'design',
    title_en: 'Design',
    title_ja: 'デザイン',
    order: 3,
    children: ['character_design', 'visual_design']
  },
  {
    key: 'music_op_ed',
    title_en: 'Music & OP/ED',
    title_ja: '音楽・OP/ED',
    order: 4,
    children: ['music', 'opening_ending']
  },
  {
    key: 'animation',
    title_en: 'Animation',
    title_ja: '作画',
    order: 5,
    children: ['animation_supervision', 'key_animation', 'inbetween_animation']
  },
  {
    key: 'art_color',
    title_en: 'Art & Color',
    title_ja: '美術・色彩',
    order: 6,
    children: ['color_design', 'art_direction']
  },
  {
    key: 'post_production',
    title_en: 'Post-Production',
    title_ja: 'ポスト',
    order: 7,
    children: ['photography', 'cg', 'editing']
  },
  {
    key: 'sound',
    title_en: 'Sound',
    title_ja: '音響',
    order: 8,
    children: ['sound_production']
  },
  {
    key: 'production_group',
    title_en: 'Production',
    title_ja: '制作',
    order: 9,
    children: ['production']
  },
]

// Map from detailed category key to parent group key
export const CATEGORY_TO_GROUP: Record<string, string> = (() => {
  const mapping: Record<string, string> = {}
  CATEGORY_GROUPS.forEach(group => {
    group.children.forEach(childKey => {
      mapping[childKey] = group.key
    })
  })
  // Also map 'other' to itself for uncategorized staff
  mapping['other'] = 'other'
  return mapping
})()

// Helper to get parent group key for any category
export function getParentGroup(categoryKey: string): string {
  return CATEGORY_TO_GROUP[categoryKey] || 'other'
}

// Get group title for display
export function getGroupTitle(group: CategoryGroup): string {
  return `${group.title_en} / ${group.title_ja}`
}

// Get detailed category by key
export function getCategoryByKey(key: string): StaffCategory | undefined {
  return STAFF_CATEGORIES.find(cat => cat.key === key)
}

// Get group by key
export function getGroupByKey(key: string): CategoryGroup | undefined {
  return CATEGORY_GROUPS.find(group => group.key === key)
}

export function categorizeStaff(staffMembers: any[]) {
  const categorized: Record<string, any[]> = {}
  const uncategorized: any[] = []

  // Initialize categories
  STAFF_CATEGORIES.forEach(category => {
    categorized[category.key] = []
  })

  staffMembers.forEach(member => {
    const roles = Array.isArray(member.role) ? member.role : [member.role]
    let assigned = false

    // Try to find matching category for each role using categorizeRole for consistency
    for (const role of roles) {
      if (!role) continue

      const categoryKey = categorizeRole(role)

      if (categoryKey !== 'other' && !assigned) {
        categorized[categoryKey].push(member)
        assigned = true
        break
      }
    }

    // If no category found, add to uncategorized (displayed as "Other Staff")
    if (!assigned) {
      uncategorized.push(member)
    }
  })

  return { categorized, uncategorized }
}

export function getCategoryTitle(category: StaffCategory): string {
  return `${category.title_en} / ${category.title_ja}`
}

export function categorizeRole(role: string): string {
  if (!role) return 'other'

  // Special case: standalone "Assistant" role should go to 'other'
  // to avoid matching compound roles like "Assistant Character Designer"
  if (role.toLowerCase() === 'assistant') return 'other'

  // First try exact match
  let category = STAFF_CATEGORIES.find(cat =>
    cat.roles.some(r => r === role)
  )

  if (category) return category.key

  // Then try partial match, but prioritize longer matches
  // and avoid matching short keywords inside parentheses
  let bestMatch: { category: StaffCategory, matchLength: number } | null = null

  for (const cat of STAFF_CATEGORIES) {
    for (const r of cat.roles) {
      // Skip very short keywords (like "OP", "ED") if they appear in parentheses
      if (r.length <= 2 && role?.includes('(') && role?.match(new RegExp(`\\(.*${r}.*\\)`))) {
        continue
      }

      if (role?.includes(r) || r?.includes(role)) {
        const matchLength = r.length
        if (!bestMatch || matchLength > bestMatch.matchLength) {
          bestMatch = { category: cat, matchLength }
        }
      }
    }
  }

  return bestMatch ? bestMatch.category.key : 'other'
}
