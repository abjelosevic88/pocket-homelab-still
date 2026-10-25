// Make or change your own breathing pattern: phases, length, ease-in.
import { useState } from 'react'
import { api, V1 } from '../lib/api'
import { breathsPerMinute, cycleSeconds, PHASE_LABEL, fmtClock, totalSeconds, cycleCount } from '../lib/player'
import type { BreathConfig, Phase, PhaseType, Preset } from '../lib/types'
import { Icon } from './icons'
import { PhaseDiagram } from './BreathVisual'
import { Confirm, Field, Modal, Segmented, Switch, errMsg, useToast } from './ui'

const TYPES: { value: PhaseType; label: string }[] = [
  { value: 'in', label: 'Breathe in' }, { value: 'in2', label: 'Top up (second sip)' }, { value: 'hold', label: 'Hold (full)' },
  { value: 'out', label: 'Breathe out' }, { value: 'hold_out', label: 'Hold (empty)' },
]

export function BreathEditor({ preset, base, onClose, onSaved }: { preset?: Preset; base?: { name: string; config: BreathConfig }; onClose: () => void; onSaved: (p: Preset | null) => void }) {
  const { toast } = useToast()
  const start = (preset?.config as BreathConfig) || base?.config || { phases: [{ type: 'in', s: 4 }, { type: 'out', s: 6 }], minutes: 5 }
  const [name, setName] = useState(preset?.name || (base ? `My ${base.name}` : ''))
  const [phases, setPhases] = useState<Phase[]>(start.phases.map(p => ({ ...p })))
  const [by, setBy] = useState<'minutes' | 'cycles'>(start.cycles ? 'cycles' : 'minutes')
  const [minutes, setMinutes] = useState(start.minutes || 5)
  const [cycles, setCycles] = useState(start.cycles || 6)
  const [ease, setEase] = useState(!!start.ease)
  const [favorite, setFavorite] = useState(preset?.favorite ?? true)
  const [busy, setBusy] = useState(false)
  const [del, setDel] = useState(false)

  const set = (i: number, p: Partial<Phase>) => setPhases(ps => ps.map((x, k) => (k === i ? { ...x, ...p } : x)))
  const move = (i: number, d: number) => setPhases(ps => { const n = [...ps]; const j = i + d; if (j < 0 || j >= n.length) return ps; [n[i], n[j]] = [n[j], n[i]]; return n })
  const config: BreathConfig = { phases: phases.map(p => ({ type: p.type, s: Number(p.s) || 0, ...(p.label?.trim() ? { label: p.label.trim() } : {}) })), ...(by === 'cycles' ? { cycles } : { minutes }), ...(ease ? { ease } : {}) }
  const cyc = cycleSeconds(config.phases)
  const n = cycleCount(config)
  const valid = name.trim() && phases.length > 0 && phases.every(p => p.s >= 0.5 && p.s <= 60) && phases.some(p => p.type === 'in' || p.type === 'out' || p.type === 'in2')

  const save = async () => {
    setBusy(true)
    try {
      const body = { kind: 'breath', name: name.trim(), config, favorite }
      const p = preset ? await api.put<Preset>(`${V1}/presets/${preset.id}`, body) : await api.post<Preset>(`${V1}/presets`, body)
      toast(preset ? 'Pattern saved' : 'Pattern added')
      onSaved(p)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const remove = async () => {
    if (!preset) return
    try { await api.del(`${V1}/presets/${preset.id}`); toast('Pattern deleted'); onSaved(null) } catch (e) { toast(errMsg(e), 'err') }
  }

  return (
    <Modal title={preset ? 'Edit pattern' : 'New breathing pattern'} onClose={onClose} size="lg" footer={<>
      {preset && <button type="button" className="btn danger ghost" onClick={() => setDel(true)}><Icon name="trash" size={15} />Delete</button>}
      <span className="spacer" />
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={!valid || busy}>{preset ? 'Save' : 'Add pattern'}</button>
    </>}>
      <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} placeholder="Evening wind-down" maxLength={80} autoFocus /></Field>
      <div className="be-phases">
        <div className="be-h"><span>Phases of one breath</span><span className="muted small">{cyc.toFixed(1).replace(/\.0$/, '')} s a breath · {breathsPerMinute(config.phases)} breaths a minute</span></div>
        {phases.map((p, i) => (
          <div key={i} className="be-row">
            <span className="be-dot" style={{ background: `var(--ph-${p.type})` }} />
            <select value={p.type} onChange={e => set(i, { type: e.target.value as PhaseType })} aria-label="Phase">
              {TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
            <div className="be-secs">
              <button type="button" className="btn ghost icon-only sm" onClick={() => set(i, { s: Math.max(0.5, Math.round((p.s - 0.5) * 2) / 2) })} aria-label="Shorter"><Icon name="minus" size={14} /></button>
              <input type="number" min={0.5} max={60} step={0.5} value={p.s} onChange={e => set(i, { s: Number(e.target.value) })} aria-label="Seconds" />
              <span className="muted small">s</span>
              <button type="button" className="btn ghost icon-only sm" onClick={() => set(i, { s: Math.min(60, p.s + 0.5) })} aria-label="Longer"><Icon name="plus" size={14} /></button>
            </div>
            <input className="be-label" value={p.label || ''} onChange={e => set(i, { label: e.target.value })} placeholder={PHASE_LABEL[p.type]} maxLength={80} aria-label="Label (optional)" />
            <div className="be-move">
              <button type="button" className="btn ghost icon-only sm" onClick={() => move(i, -1)} disabled={i === 0} aria-label="Move up"><Icon name="chevron-up" size={14} /></button>
              <button type="button" className="btn ghost icon-only sm" onClick={() => move(i, 1)} disabled={i === phases.length - 1} aria-label="Move down"><Icon name="chevron-down" size={14} /></button>
              <button type="button" className="btn ghost icon-only sm" onClick={() => setPhases(ps => ps.filter((_, k) => k !== i))} disabled={phases.length === 1} aria-label="Remove"><Icon name="x" size={14} /></button>
            </div>
          </div>
        ))}
        {phases.length < 12 && <button type="button" className="btn sm" onClick={() => setPhases(ps => [...ps, { type: ps[ps.length - 1]?.type === 'in' ? 'out' : 'in', s: 4 }])}><Icon name="plus" size={14} />Add a phase</button>}
        {phases.length > 0 && cyc > 0 && <PhaseDiagram phases={config.phases} showLabels />}
      </div>
      <div className="be-len">
        <Segmented size="sm" value={by} onChange={setBy} options={[{ value: 'minutes', label: 'Minutes' }, { value: 'cycles', label: 'Breaths' }]} label="Length by" />
        {by === 'minutes'
          ? <input type="number" min={0.5} max={120} step={0.5} value={minutes} onChange={e => setMinutes(Number(e.target.value))} aria-label="Minutes" />
          : <input type="number" min={1} max={500} value={cycles} onChange={e => setCycles(Number(e.target.value))} aria-label="Breaths" />}
        <span className="muted small">{n} breaths · {fmtClock(totalSeconds(config, n))}</span>
      </div>
      <label className="row be-switch"><Switch checked={ease} onChange={setEase} label="Ease in" /><span>Ease in <span className="muted small">— the first three breaths are shorter (good for long holds)</span></span></label>
      <label className="row be-switch"><Switch checked={favorite} onChange={setFavorite} label="On Today" /><span>Show on Today <span className="muted small">— a one-tap start</span></span></label>
      {del && <Confirm title="Delete this pattern?" message="Practices you did with it stay in your journal." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={remove} />}
    </Modal>
  )
}
