// Meditate: build a timer (length, bells, background, guidance), your saved
// timers, and the meditation styles.
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { sessionURL } from '../lib/session'
import { AMBIENT_NAMES, BELL_NAMES, bell, unlock } from '../lib/audio'
import type { Bell, MusicPlaylist, Preset, Style, TimerConfig } from '../lib/types'
import { announceChange, useLibrary } from '../components/Layout'
import { Icon } from '../components/icons'
import { Select, type SelectOption } from '../components/Select'
import { Card, Confirm, Empty, Field, Loading, Modal, PageHeader, Segmented, errMsg, useToast } from '../components/ui'

const LENGTHS = [3, 5, 10, 15, 20, 30, 45, 0]

/** The Navidrome playlists, when background music is connected. */
export function usePlaylists() {
  const [lists, setLists] = useState<MusicPlaylist[]>([])
  useEffect(() => { api.get<{ ok: boolean; playlists?: MusicPlaylist[] }>(`${V1}/music/playlists`).then(r => setLists(r.ok ? r.playlists || [] : [])).catch(() => {}) }, [])
  return lists
}

export function ambientOptions(lists: MusicPlaylist[]): SelectOption[] {
  return [
    ...Object.entries(AMBIENT_NAMES).map(([value, label]) => ({ value, label, group: 'Sounds' })),
    ...lists.map(l => ({ value: `music:${l.id}`, label: l.name, hint: `${l.songCount} songs`, group: 'Your music (Navidrome)' })),
  ]
}

const bellOptions: SelectOption[] = Object.entries(BELL_NAMES).map(([value, label]) => ({ value, label }))

function BellPick({ label, value, onChange }: { label: string; value: string; onChange: (v: Bell) => void }) {
  return (
    <Field label={label} group>
      <div className="row nowrap">
        <Select value={value || 'none'} onChange={v => onChange(v as Bell)} options={bellOptions} ariaLabel={label} />
        <button type="button" className="btn ghost icon-only" disabled={!value || value === 'none'} onClick={async () => { await unlock(); bell(value) }} aria-label={`Play ${label.toLowerCase()}`} title="Listen"><Icon name="volume" size={16} /></button>
      </div>
    </Field>
  )
}

export function defaultTimer(bellKind: string, ambient: string, minutes: number, style = 'silent'): TimerConfig {
  return { minutes, warmup_s: 10, interval_min: 0, start_bell: bellKind as Bell, interval_bell: 'chime', end_bell: bellKind as Bell, ambient: ambient || 'none', style, guidance: style === 'silent' ? 'off' : 'full' }
}

/** The timer form, shared by the builder and the preset dialog. */
function TimerForm({ c, set, styles, lists }: { c: TimerConfig; set: (c: TimerConfig) => void; styles: Style[]; lists: MusicPlaylist[] }) {
  const [custom, setCustom] = useState(!LENGTHS.includes(c.minutes))
  const up = (p: Partial<TimerConfig>) => set({ ...c, ...p })
  const st = styles.find(s => s.key === c.style)
  return (
    <div className="timer-form">
      <Field label="Length" group>
        <div className="chips">
          {LENGTHS.map(m => <button key={m} type="button" className={`chip ${!custom && c.minutes === m ? 'active' : ''}`} onClick={() => { setCustom(false); up({ minutes: m }) }}>{m ? `${m} min` : 'Open'}</button>)}
          <button type="button" className={`chip ${custom ? 'active' : ''}`} onClick={() => setCustom(true)}>Other</button>
          {custom && <input type="number" className="chip-input" min={1} max={240} value={c.minutes || ''} onChange={e => up({ minutes: Math.max(0, Math.min(240, Number(e.target.value))) })} aria-label="Minutes" />}
        </div>
      </Field>
      <div className="form-grid">
        <Field label="Style" group>
          <Select value={c.style || 'silent'} onChange={v => up({ style: v, guidance: v === 'silent' ? 'off' : c.guidance === 'off' || !c.guidance ? 'full' : c.guidance })} options={styles.map(s => ({ value: s.key, label: s.name, hint: s.tagline }))} ariaLabel="Style" />
        </Field>
        <Field label="Guidance" group help={st?.cues?.length ? 'Spoken cues and words on screen' : 'This style has no cues'}>
          <Segmented size="sm" value={c.guidance || 'off'} onChange={v => up({ guidance: v })} options={[{ value: 'off', label: 'Off' }, { value: 'light', label: 'Light', disabled: !st?.cues?.length }, { value: 'full', label: 'Full', disabled: !st?.cues?.length }]} label="Guidance" />
        </Field>
        <Field label="Warm-up" group help="Time to settle before the first bell">
          <Segmented size="sm" value={c.warmup_s} onChange={v => up({ warmup_s: v })} options={[0, 10, 30, 60].map(s => ({ value: s, label: s ? `${s}s` : 'None' }))} label="Warm-up" />
        </Field>
        <Field label="Interval bells" group help="A soft bell to come back to now and then">
          <Segmented size="sm" value={c.interval_min} onChange={v => up({ interval_min: v })} options={[0, 2, 5, 10, 15].map(s => ({ value: s, label: s ? `${s}m` : 'None' }))} label="Interval" />
        </Field>
        <BellPick label="Start bell" value={c.start_bell} onChange={v => up({ start_bell: v })} />
        <BellPick label="End bell" value={c.end_bell} onChange={v => up({ end_bell: v })} />
        {c.interval_min > 0 && <BellPick label="Interval bell" value={c.interval_bell} onChange={v => up({ interval_bell: v })} />}
        <Field label="Background" group>
          <Select value={c.ambient || 'none'} onChange={v => up({ ambient: v })} options={ambientOptions(lists)} ariaLabel="Background sound" />
        </Field>
      </div>
    </div>
  )
}

function PresetModal({ preset, initial, styles, lists, onClose, onSaved }: { preset?: Preset; initial: TimerConfig; styles: Style[]; lists: MusicPlaylist[]; onClose: () => void; onSaved: () => void }) {
  const { toast } = useToast()
  const [name, setName] = useState(preset?.name || '')
  const [c, setC] = useState<TimerConfig>((preset?.config as TimerConfig) || initial)
  const [fav, setFav] = useState(preset?.favorite ?? true)
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true)
    try {
      const body = { kind: 'timer', name: name.trim(), config: c, favorite: fav }
      if (preset) await api.put(`${V1}/presets/${preset.id}`, body)
      else await api.post(`${V1}/presets`, body)
      toast(preset ? 'Timer saved' : 'Timer added')
      onSaved()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={preset ? 'Edit timer' : 'Save as a timer'} onClose={onClose} size="lg" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={!name.trim() || busy} onClick={save}>{preset ? 'Save' : 'Add timer'}</button>
    </>}>
      <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} placeholder="Morning sit" maxLength={80} autoFocus /></Field>
      <TimerForm c={c} set={setC} styles={styles} lists={lists} />
      <label className="row be-switch"><input type="checkbox" checked={fav} onChange={e => setFav(e.target.checked)} /> Show on Today as a one-tap start</label>
    </Modal>
  )
}

function timerSummary(c: TimerConfig, styles: Style[]) {
  const st = styles.find(s => s.key === c.style)
  const parts = [c.minutes ? `${c.minutes} min` : 'open-ended', st?.name || 'Silent sit']
  if (c.interval_min) parts.push(`bell every ${c.interval_min} min`)
  if (c.ambient && c.ambient !== 'none') parts.push(c.ambient.startsWith('music:') ? 'music' : AMBIENT_NAMES[c.ambient]?.toLowerCase() || c.ambient)
  return parts.join(' · ')
}

export default function Meditate() {
  const { library, reload } = useLibrary()
  const { settings } = useApp()
  const navigate = useNavigate()
  const { toast } = useToast()
  const lists = usePlaylists()
  const [c, setC] = useState<TimerConfig | null>(null)
  const [save, setSave] = useState<{ preset?: Preset } | null>(null)
  const [del, setDel] = useState<Preset | null>(null)
  useEffect(() => {
    if (settings && !c) setC(defaultTimer(settings.bell, settings.ambient, settings.default_sit_min || 10))
  }, [settings, c])
  if (!library || !c) return <Loading />
  const styles = library.styles
  const timers = library.presets.filter(p => p.kind === 'timer')
  // the builder's settings travel in the link (cfg), so nothing is saved until you ask for it
  const startBuilder = () => navigate(`${sessionURL({ mode: 'timer', technique: c.style || 'silent', minutes: c.minutes })}&cfg=${encodeURIComponent(JSON.stringify(c))}`)
  const remove = async (p: Preset) => {
    try { await api.del(`${V1}/presets/${p.id}`); reload(); announceChange(); toast('Timer deleted') } catch (e) { toast(errMsg(e), 'err') }
  }
  const toggleFav = async (p: Preset) => {
    try { await api.put(`${V1}/presets/${p.id}`, { favorite: !p.favorite }); reload(); announceChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <div className="page meditate-page">
      <PageHeader title="Meditate" sub="Sit with bells, a background sound and as much or as little guidance as you like." />
      <Card className="timer-card" title="Timer" sub={timerSummary(c, styles)} actions={<button type="button" className="btn ghost sm" onClick={() => setSave({})}><Icon name="star" size={14} />Save as timer</button>}>
        <TimerForm c={c} set={setC} styles={styles} lists={lists} />
        <div className="timer-go"><button type="button" className="btn primary lg" onClick={startBuilder}><Icon name="play" size={16} />Begin</button></div>
      </Card>

      <Card title="Your timers" flush={timers.length > 0}>
        {timers.length === 0 ? <Empty title="No saved timers" icon="timer">Set up a timer above and choose “Save as timer” to start it with one tap.</Empty> : (
          <ul className="list">
            {timers.map(p => (
              <li key={p.id} className="list-row">
                <span className="list-ico"><Icon name="timer" size={16} /></span>
                <div className="grow"><div className="strong">{p.name}</div><div className="muted small">{timerSummary(p.config as TimerConfig, styles)}</div></div>
                <button type="button" className={`btn ghost icon-only sm star ${p.favorite ? 'on' : ''}`} onClick={() => toggleFav(p)} title={p.favorite ? 'Remove from Today' : 'Add to Today'}><Icon name="star" size={15} /></button>
                <button type="button" className="btn ghost icon-only sm" onClick={() => setSave({ preset: p })} aria-label="Edit"><Icon name="edit" size={15} /></button>
                <button type="button" className="btn ghost icon-only sm" onClick={() => setDel(p)} aria-label="Delete"><Icon name="trash" size={15} /></button>
                <button type="button" className="btn primary sm" onClick={() => navigate(sessionURL({ mode: 'timer', preset: p.id }))}><Icon name="play" size={13} />Start</button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <h2 className="group-h">Ways to sit</h2>
      <div className="style-grid">
        {styles.map(s => {
          const use = library.uses[s.key]
          return (
            <article key={s.key} className="style-card" style={{ ['--pc' as string]: s.color }}>
              <div className="pc-top"><span className="pc-ico"><Icon name={s.kind === 'body_scan' ? 'scan' : s.kind === 'loving_kindness' ? 'heart' : s.kind === 'walking' ? 'footprints' : 'lotus'} size={18} /></span><span className="pc-chip">{s.minutes} min</span>{s.cues?.length ? <span className="pc-chip ghost">Guided</span> : null}</div>
              <h3>{s.name}</h3>
              <p className="ph-tag small">{s.tagline}</p>
              <p className="muted small">{s.description}</p>
              <details className="style-how"><summary>How</summary><ol className="how">{s.how.map((h, i) => <li key={i}>{h}</li>)}</ol></details>
              <div className="pc-actions">
                {use && <span className="muted small grow">{use.count}× · {Math.round(use.minutes)} min</span>}
                <button type="button" className="btn ghost sm" onClick={() => setC({ ...c, style: s.key, minutes: s.minutes, guidance: s.cues?.length ? 'full' : 'off' })}>Use in timer</button>
                <button type="button" className="btn primary sm" onClick={() => navigate(sessionURL({ mode: 'timer', technique: s.key, minutes: s.minutes }))}><Icon name="play" size={13} />Start</button>
              </div>
            </article>
          )
        })}
      </div>
      {save && <PresetModal preset={save.preset} initial={c} styles={styles} lists={lists} onClose={() => setSave(null)} onSaved={() => { setSave(null); reload(); announceChange() }} />}
      {del && <Confirm title={`Delete “${del.name}”?`} message="Practices you did with it stay in your journal." confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={() => { const p = del; setDel(null); remove(p) }} />}
    </div>
  )
}
