package staff

import "strings"

// staffCategory maps role strings to category keys.
// Mirrors vue-app/src/utils/staffCategories.ts STAFF_CATEGORIES.
var staffCategories = []struct {
	key   string
	roles []string
}{
	{"series_direction", []string{
		"Director", "監督", "Chief Director", "総監督", "Series Director", "シリーズディレクター",
		"Assistant Director", "副監督", "助監督", "Series Episode Director", "シリーズ演出",
		"Chief Episode Director", "チーフ演出",
	}},
	{"episode_direction", []string{
		"Episode Director", "演出", "Storyboard", "絵コンテ", "Unit Director", "ユニットディレクター",
		"Assistant Episode Director", "演出補", "Storyboard Assistant", "絵コンテ補",
	}},
	{"writing", []string{
		"Original Work", "原作", "Original Story", "原案", "Series Composition", "シリーズ構成",
		"Script", "脚本", "Screenplay", "Scenario", "シナリオ", "Story", "ストーリー",
	}},
	{"character_design", []string{
		"Character Design", "キャラクターデザイン", "Original Character Design", "原案",
		"Costume Design", "コスチュームデザイン", "Character Designer", "キャラデザ",
		"Assistant Character Designer", "キャラクターデザイン協力", "キャラクターデザイン補佐",
		"Sub-Character Design", "サブキャラクターデザイン",
	}},
	{"visual_design", []string{
		"Prop Design", "プロップデザイン", "Mechanical Design", "メカニックデザイン", "メカデザイン",
		"Mecha Design", "Creature Design", "クリーチャーデザイン", "Monster Design", "モンスターデザイン",
		"Conceptual Design", "コンセプトデザイン", "World Design", "世界観設定",
		"Title Logo Design", "タイトルロゴデザイン", "ロゴデザイン",
		"Typography Design", "フォントデザイン", "フォント協力",
	}},
	{"music", []string{
		"Music", "音楽", "Composer", "作曲", "Music Composition", "音楽制作",
		"Theme Song", "主題歌", "Opening Theme", "OP", "Ending Theme", "ED",
		"Lyricist", "作詞", "Arranger", "編曲",
	}},
	{"opening_ending", []string{
		"OP Director", "OP演出", "OP Storyboard", "OP絵コンテ", "OP Animation", "OP作画",
		"ED Director", "ED演出", "ED Storyboard", "ED絵コンテ", "ED Animation", "ED作画",
		"Opening Animation Director", "OP作画監督", "Ending Animation Director", "ED作画監督",
	}},
	{"animation_supervision", []string{
		"Chief Animation Director", "総作画監督", "総作監",
		"Animation Director", "作画監督", "作監",
		"Assistant Animation Director", "作画監督補佐",
		"Action Animation Director", "アクション作画監督", "アクション作監",
		"Mechanical Animation Director", "メカ作画監督", "メカ作監", "Mecha Animation Director",
		"Effects Animation Director", "エフェクト作画監督", "エフェクト作監",
		"Layout Animation Director", "レイアウト作画監督",
		"Chief Chief Animation Director", "総総作画監督",
	}},
	{"key_animation", []string{
		"Key Animation", "原画", "一原", "Key Animator", "キーアニメーター",
		"2nd Key Animation", "第二原画", "Layout", "レイアウト", "Genga",
	}},
	{"inbetween_animation", []string{
		"In-Between Animation", "動画", "In-Between Check", "動画検査", "Douga",
	}},
	{"color_design", []string{
		"Color Design", "色彩設計", "色彩設定", "Color Designer", "Color Coordination", "色指定",
		"Color Coordinator", "Finishing", "仕上", "仕上げ", "Paint", "ペイント",
		"Digital Paint", "デジタルペイント", "Digital Coloring", "デジタル彩色",
		"Finishing Check", "仕上検査", "仕上げ検査",
	}},
	{"art_direction", []string{
		"Art Director", "美術監督", "Art Designer", "美術デザイン", "Background Art", "背景",
		"Art Design", "美術設定", "Setting", "設定", "Art Setting",
	}},
	{"photography", []string{
		"Director of Photography", "撮影監督", "Photography", "撮影", "Composite", "合成",
		"Compositing", "Special Effects", "特殊効果", "2D Works", "2Dワークス", "2Dデザイン",
		"Monitor Graphics", "モニターグラフィックス",
	}},
	{"cg", []string{
		"3DCG Director", "3DCG監督", "3DCG", "3D Animation", "3Dアニメーション",
		"CG Modeling", "CGモデリング", "3D Modeling", "3Dモデリング",
		"CG Animation", "CGアニメーション", "Rigging", "リギング",
	}},
	{"editing", []string{
		"Editing", "編集", "Editor", "Developing", "現像",
	}},
	{"sound_production", []string{
		"Sound Director", "音響監督", "Sound Production", "音響制作",
		"Sound Production Manager", "音響制作担当", "Sound Effects", "効果",
		"Recording", "録音", "Recording Studio", "録音スタジオ",
		"Audio Director", "音響ディレクター",
	}},
	{"production", []string{
		"Producer", "プロデューサー", "Executive Producer", "エグゼクティブプロデューサー",
		"Line Producer", "ラインプロデューサー", "Production", "製作",
		"Production Producer", "制作プロデューサー", "Production Desk", "制作デスク",
		"Production Manager", "制作担当", "制作マネージャー",
		"Production Assistant", "制作進行", "Design Production", "設定制作",
		"Design Manager", "設定マネージャー", "Design Control", "設定管理",
		"General Planning", "総合企画", "Planning", "企画",
		"Animation Production", "制作", "Production Cooperation", "制作協力",
		"Animation Production Cooperation", "アニメーション制作協力",
		"Production Committee / Copyright", "製作・著作",
	}},
}

// categorizeRole returns the category key for a role string.
// Matches vue-app/src/utils/staffCategories.ts categorizeRole().
func categorizeRole(role string) string {
	if role == "" {
		return "other"
	}
	if strings.ToLower(role) == "assistant" {
		return "other"
	}

	// Exact match first.
	for _, cat := range staffCategories {
		for _, r := range cat.roles {
			if r == role {
				return cat.key
			}
		}
	}

	// Partial match — prefer longest match.
	var bestKey string
	var bestLen int
	for _, cat := range staffCategories {
		for _, r := range cat.roles {
			if len(r) <= 2 && strings.Contains(role, "(") {
				// Skip short keywords in parentheses.
				continue
			}
			if strings.Contains(role, r) || strings.Contains(r, role) {
				if len(r) > bestLen {
					bestKey = cat.key
					bestLen = len(r)
				}
			}
		}
	}

	if bestKey != "" {
		return bestKey
	}
	return "other"
}
