// How every page starts a practice: a link to the full-screen player at
// /session with what to play in the query string.
import type { LessonPractice } from './types'

export interface SessionSpec {
  mode: 'breath' | 'timer'
  technique?: string // a pattern key (breath) or a meditation style key (timer)
  preset?: number // one of the person's own presets; its config wins
  minutes?: number // length; 0 = an open-ended timer
  cycles?: number // breathing cycles instead of minutes
  program?: string // practising a lesson
  lesson?: string
}

export function sessionURL(s: SessionSpec): string {
  const q = new URLSearchParams({ mode: s.mode })
  if (s.technique) q.set('technique', s.technique)
  if (s.preset) q.set('preset', String(s.preset))
  if (s.minutes !== undefined) q.set('min', String(s.minutes))
  if (s.cycles) q.set('cycles', String(s.cycles))
  if (s.program && s.lesson) { q.set('program', s.program); q.set('lesson', s.lesson) }
  return `/session?${q.toString()}`
}

export function parseSession(q: URLSearchParams): SessionSpec {
  const n = (k: string) => { const v = q.get(k); return v === null || v === '' ? undefined : Number(v) }
  return {
    mode: q.get('mode') === 'breath' ? 'breath' : 'timer',
    technique: q.get('technique') || undefined,
    preset: n('preset'),
    minutes: n('min'),
    cycles: n('cycles'),
    program: q.get('program') || undefined,
    lesson: q.get('lesson') || undefined,
  }
}

/** The session for a lesson's practice. */
export function lessonSession(p: LessonPractice, program: string, lesson: string): SessionSpec {
  return { mode: p.kind === 'breath' ? 'breath' : 'timer', technique: p.technique, minutes: p.minutes, program, lesson }
}
