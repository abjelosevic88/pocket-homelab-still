// The timing of a breathing exercise: where in which cycle and phase a
// moment falls, and how full the lungs are (0 empty … 1 full) for the visual.
import type { BreathConfig, Phase } from './types'

export const PHASE_LABEL: Record<string, string> = { in: 'Breathe in', in2: 'Top up', hold: 'Hold', out: 'Breathe out', hold_out: 'Hold' }
export const PHASE_SPOKEN: Record<string, string> = { in: 'In', in2: 'And again', hold: 'Hold', out: 'Out', hold_out: 'Hold' }
export const PHASE_COLOR: Record<string, string> = { in: '#38bdf8', in2: '#22d3ee', hold: '#a78bfa', out: '#34d399', hold_out: '#94a3b8' }

export const cycleSeconds = (phases: Phase[]) => phases.reduce((t, p) => t + p.s, 0)

/** Ease stretches the first three cycles from 60 % to the full pattern. */
export function cycleScale(cycle: number, ease?: boolean) {
  if (!ease) return 1
  return [0.6, 0.73, 0.87][cycle] ?? 1
}

/** How many cycles a config runs: its own count, or as many as fit the minutes. */
export function cycleCount(c: BreathConfig, minutes?: number, cycles?: number): number {
  if (cycles && cycles > 0) return Math.round(cycles)
  const full = cycleSeconds(c.phases) || 1
  const m = minutes ?? c.minutes
  if (m && m > 0) {
    let t = 0, n = 0
    while (t < m * 60 - full * 0.4 && n < 2000) { t += full * cycleScale(n, c.ease); n++ }
    return Math.max(1, n)
  }
  return Math.max(1, c.cycles || Math.round(300 / full))
}

export function totalSeconds(c: BreathConfig, n: number): number {
  const full = cycleSeconds(c.phases)
  let t = 0
  for (let i = 0; i < n; i++) t += full * cycleScale(i, c.ease)
  return t
}

/** The level the lungs reach at the end of each phase. */
export function phaseLevels(phases: Phase[]): { from: number; to: number }[] {
  const out: { from: number; to: number }[] = []
  let lvl = 0
  phases.forEach((p, i) => {
    let to = lvl
    if (p.type === 'in') to = phases[i + 1]?.type === 'in2' ? 0.8 : 1
    else if (p.type === 'in2') to = 1
    else if (p.type === 'out') to = 0
    out.push({ from: lvl, to })
    lvl = to
  })
  // a pattern that starts holding full lungs starts full
  if (out.length && phases[0].type === 'hold') out[0] = { from: 1, to: 1 }
  return out
}

export interface BreathMoment {
  done: boolean
  cycle: number // 0-based
  index: number // phase index
  phase: Phase
  phaseDur: number
  phaseLeft: number
  progress: number // 0..1 within the phase
  level: number // 0..1
}

const ease = (x: number) => 0.5 - Math.cos(Math.PI * x) / 2

/** Where the exercise is after `t` seconds of breathing. */
export function momentAt(c: BreathConfig, n: number, t: number): BreathMoment {
  const levels = phaseLevels(c.phases)
  let acc = 0
  for (let cy = 0; cy < n; cy++) {
    const k = cycleScale(cy, c.ease)
    for (let i = 0; i < c.phases.length; i++) {
      const d = c.phases[i].s * k
      if (t < acc + d) {
        const pr = (t - acc) / d
        const l = levels[i]
        return { done: false, cycle: cy, index: i, phase: c.phases[i], phaseDur: d, phaseLeft: acc + d - t, progress: pr, level: l.from + (l.to - l.from) * ease(pr) }
      }
      acc += d
    }
  }
  const last = c.phases.length - 1
  return { done: true, cycle: n - 1, index: last, phase: c.phases[last], phaseDur: 0, phaseLeft: 0, progress: 1, level: levels[last]?.to ?? 0 }
}

export function fmtClock(sec: number): string {
  const s = Math.max(0, Math.round(sec))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const r = s % 60
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}` : `${m}:${String(r).padStart(2, '0')}`
}

/** "4 · 7 · 8" */
export const patternCounts = (phases: Phase[]) => phases.map(p => (Number.isInteger(p.s) ? p.s : p.s.toFixed(1))).join(' · ')

export const breathsPerMinute = (phases: Phase[]) => {
  const s = cycleSeconds(phases)
  return s > 0 ? Math.round((60 / s) * 10) / 10 : 0
}
