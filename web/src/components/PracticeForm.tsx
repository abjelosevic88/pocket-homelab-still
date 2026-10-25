// Add a practice by hand (done away from the app) or edit one.
import React, { useState } from 'react'
import { api, V1 } from '../lib/api'
import type { Practice, PracticeResult } from '../lib/types'
import { CALM_LABEL, KIND_NAME } from '../lib/still'
import { useLibrary } from './Layout'
import { Select } from './Select'
import { Field, Modal, Segmented, useToast } from './ui'

function localInput(iso?: string): string {
  const d = iso ? new Date(iso) : new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}

export function CalmPicker({ value, onChange, label }: { value: number | null; onChange: (v: number | null) => void; label: string }) {
  return (
    <div className="calm-picker" role="radiogroup" aria-label={label}>
      {[1, 2, 3, 4, 5].map(n => (
        <button key={n} type="button" role="radio" aria-checked={value === n} className={`calm-dot c${n} ${value === n ? 'on' : ''}`} title={CALM_LABEL[n]} onClick={() => onChange(value === n ? null : n)}>{n}</button>
      ))}
      <span className="muted small">{value ? CALM_LABEL[value] : 'not rated'}</span>
    </div>
  )
}

export function PracticeModal({ onClose, onSaved, practice }: { onClose: () => void; onSaved: (p: Practice) => void; practice?: Practice }) {
  const { library } = useLibrary()
  const { toast } = useToast()
  const [f, setF] = useState({
    kind: practice?.kind || 'meditation', technique: practice?.technique || '', title: practice?.title || '',
    at: localInput(practice ? practice.started_at : new Date(Date.now() - 10 * 60000).toISOString()),
    minutes: practice ? String(Math.round((practice.duration_s / 60) * 10) / 10) : '10',
    calm_before: practice?.calm_before ?? null as number | null, calm_after: practice?.calm_after ?? null as number | null,
    note: practice?.note || '', tags: practice?.tags || '', completed: practice?.completed ?? true,
  })
  const [busy, setBusy] = useState(false)
  const techOptions = [
    { value: '', label: 'None / other' },
    ...(library?.patterns || []).map(p => ({ value: p.key, label: p.name, group: 'Breathing' })),
    ...(library?.styles || []).map(s => ({ value: s.key, label: s.name, group: 'Meditation' })),
    ...(library?.presets || []).map(p => ({ value: `preset:${p.id}`, label: p.name, group: 'Yours' })),
  ]
  const pickTech = (v: string) => {
    const p = library?.patterns.find(x => x.key === v), s = library?.styles.find(x => x.key === v), pr = library?.presets.find(x => `preset:${x.id}` === v)
    setF({ ...f, technique: v, kind: p ? 'breath' : s ? s.kind : pr ? (pr.kind === 'breath' ? 'breath' : 'meditation') : f.kind })
  }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const min = Number(f.minutes)
    if (!(min > 0) || min > 360) { toast('Minutes: 0 to 360', 'err'); return }
    setBusy(true)
    const body = { kind: f.kind, technique: f.technique, title: f.title, started_at: new Date(f.at).toISOString(), duration_s: Math.round(min * 60), calm_before: f.calm_before, calm_after: f.calm_after, note: f.note, tags: f.tags, completed: f.completed }
    try {
      const r = practice
        ? await api.put<PracticeResult>(`${V1}/practices/${practice.id}`, body)
        : await api.post<PracticeResult>(`${V1}/practices`, { ...body, planned_s: Math.round(min * 60), source: 'manual' })
      toast(r.unlocked?.length ? `Saved · new milestone: ${r.unlocked.map(a => a.name).join(', ')}` : 'Saved')
      onSaved(r.practice)
    } catch (err) { toast((err as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={practice ? 'Edit practice' : 'Add a practice'} onClose={onClose}>
      <form onSubmit={save} className="grid" style={{ gap: 14 }}>
        <Field group label="What">
          <Select ariaLabel="Technique" value={f.technique} onChange={pickTech} options={techOptions} placeholder="Choose a technique…" />
        </Field>
        <Field group label="Kind">
          <Select ariaLabel="Kind" value={f.kind} onChange={v => setF({ ...f, kind: v })} options={Object.entries(KIND_NAME).map(([value, label]) => ({ value, label }))} />
        </Field>
        <Field label="Title (optional)" help="Shown in the journal; defaults to the technique."><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} maxLength={200} /></Field>
        <div className="two-col tight">
          <Field label="Started"><input type="datetime-local" value={f.at} onChange={e => setF({ ...f, at: e.target.value })} required /></Field>
          <Field label="Minutes"><input type="number" min={0.5} max={360} step={0.5} value={f.minutes} onChange={e => setF({ ...f, minutes: e.target.value })} required /></Field>
        </div>
        <Field group label="Before: how did you feel?"><CalmPicker label="Before" value={f.calm_before} onChange={v => setF({ ...f, calm_before: v })} /></Field>
        <Field group label="After"><CalmPicker label="After" value={f.calm_after} onChange={v => setF({ ...f, calm_after: v })} /></Field>
        <Field label="Note"><textarea rows={3} value={f.note} onChange={e => setF({ ...f, note: e.target.value })} placeholder="What came up, what helped…" /></Field>
        <Field label="Tags" help="Comma separated: morning, work, sleep…"><input value={f.tags} onChange={e => setF({ ...f, tags: e.target.value })} /></Field>
        {practice && <Field group label="Finished"><Segmented value={f.completed ? 'y' : 'n'} onChange={v => setF({ ...f, completed: v === 'y' })} options={[{ value: 'y', label: 'Finished' }, { value: 'n', label: 'Stopped early' }]} size="sm" /></Field>}
        <div className="form-actions"><button type="button" className="btn" onClick={onClose}>Cancel</button><button className="btn primary" disabled={busy}>{practice ? 'Save' : 'Add practice'}</button></div>
      </form>
    </Modal>
  )
}
