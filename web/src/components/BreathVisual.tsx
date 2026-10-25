// What you breathe along with: an orb that grows and shrinks, a dot going
// round a box, or a wave; plus the small phase diagram on the pattern cards.
import type { Phase } from '../lib/types'
import { type BreathMoment, PHASE_COLOR, PHASE_LABEL, cycleSeconds, phaseLevels } from '../lib/player'

export function BreathVisual({ visual, m, phases }: { visual: 'orb' | 'box' | 'wave'; m: BreathMoment; phases: Phase[] }) {
  if (visual === 'box') return <BoxVisual m={m} phases={phases} />
  if (visual === 'wave') return <WaveVisual m={m} phases={phases} />
  const scale = 0.42 + 0.58 * m.level
  return (
    <div className="bv bv-orb" aria-hidden="true">
      <div className="bv-orb-halo" style={{ transform: `scale(${0.55 + 0.6 * m.level})`, opacity: 0.25 + 0.35 * m.level }} />
      <div className="bv-orb-ring" />
      <div className="bv-orb-core" style={{ transform: `scale(${scale})` }} />
    </div>
  )
}

function BoxVisual({ m, phases }: { m: BreathMoment; phases: Phase[] }) {
  // the perimeter is split into one side per phase (a 4-phase box gets four equal sides)
  const n = phases.length
  const pos = (m.index + m.progress) / n
  const S = 200, P = 20
  const pts = n === 4 ? [[P, S - P], [P, P], [S - P, P], [S - P, S - P]] : null
  let x = S / 2, y = S / 2
  if (pts) {
    const side = Math.min(3, Math.floor(pos * 4))
    const f = pos * 4 - side
    const a = pts[side], b = pts[(side + 1) % 4]
    x = a[0] + (b[0] - a[0]) * f
    y = a[1] + (b[1] - a[1]) * f
  } else {
    const ang = -Math.PI / 2 + pos * 2 * Math.PI
    x = S / 2 + Math.cos(ang) * (S / 2 - P)
    y = S / 2 + Math.sin(ang) * (S / 2 - P)
  }
  return (
    <div className="bv bv-box" aria-hidden="true">
      <svg viewBox={`0 0 ${S} ${S}`}>
        {pts ? <rect x={P} y={P} width={S - 2 * P} height={S - 2 * P} rx="14" className="bv-track" /> : <circle cx={S / 2} cy={S / 2} r={S / 2 - P} className="bv-track" />}
        <circle cx={x} cy={y} r={9 + 5 * m.level} className="bv-dot" />
        <circle cx={S / 2} cy={S / 2} r={18 + 40 * m.level} className="bv-fill" />
      </svg>
    </div>
  )
}

function WaveVisual({ m, phases }: { m: BreathMoment; phases: Phase[] }) {
  const W = 320, H = 160, pad = 14
  const total = cycleSeconds(phases) || 1
  const levels = phaseLevels(phases)
  const pts: string[] = []
  let acc = 0
  phases.forEach((p, i) => {
    const steps = 16
    for (let k = 0; k <= steps; k++) {
      const f = k / steps
      const l = levels[i].from + (levels[i].to - levels[i].from) * (0.5 - Math.cos(Math.PI * f) / 2)
      pts.push(`${pad + ((acc + p.s * f) / total) * (W - 2 * pad)},${H - pad - l * (H - 2 * pad)}`)
    }
    acc += p.s
  })
  let before = 0
  for (let i = 0; i < m.index; i++) before += phases[i].s
  const cx = pad + ((before + phases[m.index].s * m.progress) / total) * (W - 2 * pad)
  const cy = H - pad - m.level * (H - 2 * pad)
  return (
    <div className="bv bv-wave" aria-hidden="true">
      <svg viewBox={`0 0 ${W} ${H}`}>
        <polyline points={pts.join(' ')} className="bv-track" />
        <line x1={cx} x2={cx} y1={pad} y2={H - pad} className="bv-cursor" />
        <circle cx={cx} cy={cy} r={10} className="bv-dot" />
      </svg>
    </div>
  )
}

/** A strip of the phases in proportion, with a cursor running at real speed. */
export function PhaseDiagram({ phases, animate = true, showLabels = false }: { phases: Phase[]; animate?: boolean; showLabels?: boolean }) {
  const total = cycleSeconds(phases) || 1
  return (
    <div className={`phase-diagram ${showLabels ? 'labels' : ''}`} aria-label={phases.map(p => `${p.label || PHASE_LABEL[p.type]} ${p.s}s`).join(', ')}>
      <div className="pd-bar">
        {phases.map((p, i) => (
          <span key={i} className={`pd-seg pd-${p.type}`} style={{ flexGrow: p.s, background: PHASE_COLOR[p.type] }} title={`${p.label || PHASE_LABEL[p.type]} · ${p.s}s`}>
            {showLabels && <em>{p.s}s</em>}
          </span>
        ))}
        {animate && <i className="pd-cursor" style={{ animationDuration: `${total}s` }} />}
      </div>
      {showLabels && (
        <div className="pd-legend">
          {phases.map((p, i) => <span key={i} style={{ flexGrow: p.s }}>{p.label || PHASE_LABEL[p.type]}</span>)}
        </div>
      )}
    </div>
  )
}
