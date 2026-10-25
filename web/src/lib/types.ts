export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'

export type Bell = 'bowl' | 'bell' | 'chime' | 'gong' | 'wood' | 'none'
export type PlayerTheme = 'dusk' | 'ocean' | 'forest' | 'night' | 'minimal'

export interface Settings {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  daily_goal_min: number; weekly_goal_days: number
  bell: Bell; ambient: string; breath_sound: 'soft' | 'bell' | 'none'; volume: number; voice: boolean; voice_name: string
  haptics: boolean; ask_calm: boolean; keep_awake: boolean; player_theme: PlayerTheme; breath_visual: 'orb' | 'box' | 'wave'; default_sit_min: number
  reminder_times: string[]; reminder_days: string; reminder_skip_done: boolean
  notify_streak: boolean; streak_hour: number; notify_milestones: boolean; notify_weekly: boolean; weekly_day: number; weekly_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  habits_mode: ConnMode; habits_via: ConnVia; habits_detail: string; habits_open_url: string; habits_configured: boolean
  habits_url: string; habits_token: string; habits_public_url: string
  health_mode: ConnMode; health_via: ConnVia; health_detail: string; health_open_url: string; health_configured: boolean
  health_url: string; health_token: string; health_public_url: string; health_sync: boolean; health_insights: boolean
  music_mode: ConnMode; music_url: string; music_user: string; music_password: string
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string
}

// ---- the library ----

export type PhaseType = 'in' | 'in2' | 'hold' | 'out' | 'hold_out'
export interface Phase { type: PhaseType; s: number; label?: string }
export interface BreathConfig { phases: Phase[]; minutes?: number; cycles?: number; ease?: boolean; sound?: string; ambient?: string; voice?: boolean }
export interface TimerConfig {
  minutes: number; warmup_s: number; interval_min: number; start_bell: Bell | ''; interval_bell: Bell | ''; end_bell: Bell | ''
  ambient: string; style: string; guidance: '' | 'off' | 'light' | 'full'
}
export interface Pattern {
  key: string; name: string; tagline: string; description: string; how: string[]
  good_for: 'calm' | 'focus' | 'sleep' | 'energy' | 'balance'; level: 'beginner' | 'intermediate'; color: string; caution?: string; config: BreathConfig
}
export interface Cue { at: number; text: string; key?: boolean }
export interface Style { key: string; name: string; kind: string; tagline: string; description: string; how: string[]; minutes: number; color: string; cues: Cue[] | null }
export interface Kind { key: string; name: string; icon: string }
export interface Preset { id: number; kind: 'breath' | 'timer'; name: string; config: BreathConfig | TimerConfig; favorite: boolean; sort: number; created_at: string; updated_at: string }
export interface TechniqueUse { count: number; minutes: number; last: string }
export interface Library {
  patterns: Pattern[]; styles: Style[]; kinds: Kind[]; bells: Bell[]; ambients: string[]
  presets: Preset[]; favorites: Record<string, boolean>; uses: Record<string, TechniqueUse>
}

// ---- practices ----

export interface Practice {
  id: number; kind: string; technique: string; title: string; started_at: string; ended_at: string; date: string
  duration_s: number; planned_s: number; completed: boolean; cycles: number
  calm_before: number | null; calm_after: number | null; hr_before: number | null; hr_after: number | null
  note: string; tags: string; program: string; lesson: string; source: string; external_id: string
  health_synced_at: string; health_error: string; habit_marked_at: string; habit_error: string; created_at: string; updated_at: string
}
export interface Achievement { key: string; name: string; description: string; icon: string; group: string }
export interface Level { n: number; name: string; minutes: number; blurb: string }

/** What POST /practices answers: the saved practice plus what it set off. */
export interface PracticeResult {
  practice: Practice; unlocked: Achievement[]; lesson_completed?: boolean; program?: ProgramView
  day_minutes: number; goal_min: number; streak: number
}

// ---- programs ----

export interface LessonPractice { kind: string; technique: string; minutes: number }
export interface LessonView {
  key: string; title: string; teaching: string[]; tip?: string; practice: LessonPractice
  program: string; stage: string; stage_idx: number; index: number; done: boolean; done_at?: string; locked: boolean; current: boolean
}
export interface StageView { key: string; name: string; tagline: string; lessons: LessonView[]; done: number; total: number; complete: boolean; locked: boolean; minutes: number }
export interface ProgramView {
  key: string; name: string; kind: 'path' | 'course'; tagline: string; description: string; color: string; icon: string
  done: number; total: number; percent: number; started_at?: string; completed_at?: string; last_at?: string
  stage_idx: number; stage_name: string; next?: LessonView; stages?: StageView[]
}

// ---- home, stats, milestones ----

export interface Suggestion { key: string; name: string; reason: string; part_of_day: string; type: 'pattern' | 'style'; color: string; minutes: number }
export interface QuickItem { type: 'pattern' | 'style' | 'preset'; key: string; id?: number; kind?: 'breath' | 'timer'; name: string; color?: string; minutes?: number; cycles?: number; config?: BreathConfig | TimerConfig }
export interface HomeData {
  part_of_day: string; intention: string
  today: { date: string; minutes: number; sessions: number; goal_min: number; goal_pct: number }
  week: { date: string; minutes: number; met: boolean }[]
  streak: number; longest_streak: number
  lifetime: { minutes: number; sessions: number; level: Level; next_level: Level | null; level_pct: number }
  next_lesson: LessonView | null; next_program: ProgramView | null; in_progress: ProgramView[] | null
  recent: Practice[]; today_practices: Practice[]; suggestion: Suggestion; quick: QuickItem[]
  latest_milestones: (Achievement & { unlocked_at: string })[] | null
}
export interface Dashboard {
  today_min: number; goal_min: number; goal_met: boolean; week_min: number; week_days: number; weekly_goal_days: number
  streak: number; longest_streak: number; lifetime_min: number; sessions: number; level: string
  path_done?: number; path_total?: number; path_stage?: string; next_lesson?: string
}
export interface TechStat { key: string; name: string; kind: string; sessions: number; minutes: number; rated: number; avg_before: number; avg_after: number; avg_shift: number; last_date: string; completion_pct: number }
export interface PartStat { sessions: number; minutes: number; avg_shift: number }
export interface Stats {
  range: { from: string; to: string; days: number }
  totals: { minutes: number; sessions: number; days_practiced: number; avg_session_min: number; longest_session_min: number; consistency_pct: number; avg_daily_min: number; completion_pct: number }
  previous: { from: string; to: string; minutes: number; sessions: number; days_practiced: number; change_pct?: number }
  lifetime: { minutes: number; sessions: number; days: number; first_date: string; level: Level; next_level: Level | null; level_pct: number; minutes_to_next?: number }
  streak: { current: number; longest: number; goal_current: number; goal_longest: number }
  daily: { date: string; minutes: number; sessions: number; goal_met: boolean }[]
  weekly: { week: string; minutes: number; days: number }[]
  monthly: { month: string; minutes: number; sessions: number; days: number }[]
  heatmap: { date: string; minutes: number }[]
  by_kind: { kind: string; name: string; sessions: number; minutes: number }[]
  by_technique: TechStat[]
  by_hour: { hour: number; sessions: number; minutes: number }[]
  parts_of_day: Record<'morning' | 'afternoon' | 'evening' | 'night', PartStat>
  weekdays: { weekday: number; days: number; practiced: number; sessions: number; avg_minutes: number; practice_pct: number }[]
  lengths: { label: string; sessions: number }[]
  mood: { rated: number; with_before: number; avg_before: number; avg_after: number; avg_shift: number; improved_pct: number; weekly: { week: string; avg_after: number; avg_shift: number; rated: number }[] }
  goals: { daily_goal_min: number; weekly_goal_days: number; days_met: number; days_met_pct: number; this_week_days: number; this_week_minutes: number; week_start: string }
  insights: string[]
}
export interface HealthMetric {
  key: string; name: string; unit: string; higher_is_better: boolean; when: 'same_day' | 'next_night'
  practice_avg: number; rest_avg: number; diff: number; diff_pct: number; n_practice: number; n_rest: number; correlation: number; better?: boolean
  points: { date: string; value: number; minutes: number }[]
}
export interface HealthInsights { ok: boolean; error?: string; not_connected?: boolean; via?: string; open_url?: string; range?: string; from?: string; to?: string; metrics?: HealthMetric[]; errors?: string[]; note?: string }
export interface AchievementItem extends Achievement { unlocked: boolean; unlocked_at?: string }
export interface Achievements { achievements: AchievementItem[]; unlocked: number; total: number; levels: Level[]; level: Level; next_level: Level | null; lifetime_min: number }

// ---- connections ----

export interface HabitRule { id: number; kind: string; min_minutes: number; habit_id: number; habit_name: string; enabled: boolean; created_at: string }
export interface Habit { id: number; name: string; color: string; days: string }
export interface MusicPlaylist { id: string; name: string; songCount: number; duration: number; comment?: string }
export interface MusicSong { id: string; title: string; artist: string; album: string; duration: number }
