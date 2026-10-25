// Small shared bits for the practice pages: names, icons, how minutes and calm read.
import type { Library, Practice } from './types'

export const KIND_ICON: Record<string, string> = { breath: 'wind', meditation: 'lotus', body_scan: 'scan', loving_kindness: 'heart', walking: 'footprints', other: 'sparkles' }
export const KIND_COLOR: Record<string, string> = { breath: '#0ea5e9', meditation: '#0f766e', body_scan: '#14b8a6', loving_kindness: '#ec4899', walking: '#84cc16', other: '#64748b' }
export const KIND_NAME: Record<string, string> = { breath: 'Breathing', meditation: 'Meditation', body_scan: 'Body scan', loving_kindness: 'Loving-kindness', walking: 'Mindful walk', other: 'Other' }
export const CALM_LABEL = ['', 'Tense', 'Uneasy', 'Okay', 'Settled', 'Calm']
export const GROUP_NAME: Record<string, string> = { start: 'First steps', streak: 'Streaks', time: 'Time', sessions: 'Practices', explore: 'Exploring', moments: 'Moments', path: 'The Path and courses' }

/** "12 min", "1 h 5 min", "45 s" */
export function fmtMin(min: number): string {
  if (!min) return '0 min'
  if (min < 1) return `${Math.round(min * 60)} s`
  const m = Math.round(min)
  const h = Math.floor(m / 60), r = m % 60
  return h ? (r ? `${h} h ${r} min` : `${h} h`) : `${m} min`
}

export const practiceMin = (p: Practice) => p.duration_s / 60

/** The name of a technique key: a built-in pattern or style, or one of your presets. */
export function techniqueName(lib: Library | null, key: string): string {
  if (!key) return ''
  const p = lib?.patterns.find(x => x.key === key)
  if (p) return p.name
  const s = lib?.styles.find(x => x.key === key)
  if (s) return s.name
  if (key.startsWith('preset:')) return lib?.presets.find(x => x.id === Number(key.slice(7)))?.name || 'Your own'
  return ''
}

export function techniqueColor(lib: Library | null, key: string, kind = ''): string {
  return lib?.patterns.find(x => x.key === key)?.color || lib?.styles.find(x => x.key === key)?.color || KIND_COLOR[kind] || 'var(--accent)'
}

/** "Good morning" etc. */
export function greeting(part: string): string {
  return part === 'morning' ? 'Good morning' : part === 'afternoon' ? 'Good afternoon' : part === 'evening' ? 'Good evening' : 'Good night'
}

export const WEEKDAYS = ['', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
