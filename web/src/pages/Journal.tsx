// The journal: every practice, filtered and grouped by day; one practice with its details.
import { type ReactNode, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { sessionURL } from '../lib/session'
import type { Practice } from '../lib/types'
import { ago, fmtDate, fmtDateTime, fmtISOTime } from '../lib/format'
import { CALM_LABEL, KIND_ICON, KIND_NAME, fmtMin, practiceMin, techniqueColor, techniqueName } from '../lib/still'
import { Card, Confirm, Empty, Loading, PageHeader, useAsync, useDebounce, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { PracticeModal } from '../components/PracticeForm'
import { PracticeRow } from './Today'
import { announceChange, openCreate, useCrumb, useLibrary, useOnChange } from '../components/Layout'

const PAGE = 50

export default function Journal() {
  const { library } = useLibrary()
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 300)
  const [kind, setKind] = useState('')
  const [tech, setTech] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [notes, setNotes] = useState(false)
  const [items, setItems] = useState<Practice[] | null>(null)
  const [total, setTotal] = useState(0)
  const [busy, setBusy] = useState(false)
  const params = (offset: number) => {
    const p = new URLSearchParams({ limit: String(PAGE), offset: String(offset) })
    if (dq) p.set('q', dq); if (kind) p.set('kind', kind); if (tech) p.set('technique', tech)
    if (from) p.set('from', from); if (to) p.set('to', to); if (notes) p.set('notes', '1')
    return p.toString()
  }
  const load = (offset = 0) => {
    setBusy(true)
    api.get<{ practices: Practice[]; total: number }>(`${V1}/practices?${params(offset)}`)
      .then(r => { setItems(prev => offset ? [...(prev || []), ...r.practices] : r.practices); setTotal(r.total) })
      .catch(() => setItems([])).finally(() => setBusy(false))
  }
  useEffect(() => { load(0) }, [dq, kind, tech, from, to, notes]) // eslint-disable-line react-hooks/exhaustive-deps
  useOnChange(() => load(0))
  const groups: [string, Practice[]][] = []
  for (const p of items || []) {
    const g = groups[groups.length - 1]
    if (g && g[0] === p.date) g[1].push(p); else groups.push([p.date, [p]])
  }
  const filtered = !!(dq || kind || tech || from || to || notes)
  const techOptions = [{ value: '', label: 'Any technique' }, ...(library?.patterns || []).map(p => ({ value: p.key, label: p.name, group: 'Breathing' })), ...(library?.styles || []).map(s => ({ value: s.key, label: s.name, group: 'Meditation' })), ...(library?.presets || []).map(p => ({ value: `preset:${p.id}`, label: p.name, group: 'Yours' }))]
  return (
    <div className="stack loose">
      <PageHeader title="Journal" sub={items ? `${total.toLocaleString()} practice${total === 1 ? '' : 's'}${filtered ? ' match' : ''}` : undefined}
        actions={<>
          <a className="btn ghost sm" href={`${V1}/export.csv`}><Icon name="download" size={14} />CSV</a>
          <a className="btn ghost sm" href={`${V1}/export.json`}><Icon name="download" size={14} />JSON</a>
          <button type="button" className="btn primary sm" onClick={() => openCreate('practice')}><Icon name="plus" size={14} />Add</button>
        </>} />
      <div className="row wrap filters">
        <div className="search-box"><Icon name="search" size={15} /><input value={q} onChange={e => setQ(e.target.value)} placeholder="Search notes, titles, tags…" aria-label="Search" /></div>
        <Select ariaLabel="Kind" value={kind} onChange={setKind} options={[{ value: '', label: 'Every kind' }, ...Object.entries(KIND_NAME).map(([value, label]) => ({ value, label }))]} />
        <Select ariaLabel="Technique" value={tech} onChange={setTech} options={techOptions} />
        <input type="date" value={from} onChange={e => setFrom(e.target.value)} aria-label="From" className="date-in" />
        <input type="date" value={to} onChange={e => setTo(e.target.value)} aria-label="To" className="date-in" />
        <label className="check-row"><input type="checkbox" checked={notes} onChange={e => setNotes(e.target.checked)} />With notes</label>
        {filtered && <button type="button" className="btn ghost sm" onClick={() => { setQ(''); setKind(''); setTech(''); setFrom(''); setTo(''); setNotes(false) }}>Clear</button>}
      </div>
      {!items ? <Loading /> : items.length === 0 ? (
        <Empty title={filtered ? 'Nothing matches' : 'No practices yet'} icon="journal">{filtered ? 'Try fewer filters.' : 'Every breath and sit you finish lands here, with how you felt before and after.'}</Empty>
      ) : (
        <Card flush>
          <div className="list">
            {groups.map(([d, ps]) => (
              <div key={d}>
                <div className="day-h row"><span className="grow">{fmtDate(d)}</span><span className="tnum">{fmtMin(ps.reduce((a, p) => a + practiceMin(p), 0))}</span></div>
                {ps.map(p => <PracticeRow key={p.id} p={p} />)}
              </div>
            ))}
          </div>
        </Card>
      )}
      {items && items.length < total && <div className="row"><button type="button" className="btn" disabled={busy} onClick={() => load(items.length)}>{busy ? 'Loading…' : `Show more (${total - items.length})`}</button></div>}
    </div>
  )
}

export function PracticePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { library } = useLibrary()
  const { data: p, setData, error } = useAsync(() => api.get<Practice>(`${V1}/practices/${id}`), [id])
  const [edit, setEdit] = useState(false)
  const [del, setDel] = useState(false)
  useCrumb(p?.title)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!p) return <Loading />
  const color = techniqueColor(library, p.technique, p.kind)
  const tname = techniqueName(library, p.technique)
  const again = () => {
    if (p.technique.startsWith('preset:')) navigate(sessionURL({ mode: p.kind === 'breath' ? 'breath' : 'timer', preset: Number(p.technique.slice(7)) }))
    else navigate(sessionURL({ mode: p.kind === 'breath' ? 'breath' : 'timer', technique: p.technique || 'silent', minutes: p.kind === 'breath' ? undefined : Math.round(p.duration_s / 60) || 10 }))
  }
  const remove = async () => { setDel(false); try { await api.del(`${V1}/practices/${p.id}`); announceChange(); toast('Practice deleted'); navigate('/journal') } catch (e) { toast((e as Error).message, 'err') } }
  const rows: [string, ReactNode][] = [
    ['When', `${fmtDateTime(p.started_at)}${p.ended_at ? ` – ${fmtISOTime(p.ended_at)}` : ''}`],
    ['Length', <span key="l">{fmtMin(practiceMin(p))}{p.planned_s > 0 && p.planned_s !== p.duration_s ? <span className="muted"> of {fmtMin(p.planned_s / 60)} planned</span> : null}{!p.completed && <span className="badge warn">stopped early</span>}</span>],
    ['Kind', KIND_NAME[p.kind] || p.kind],
    ...(tname ? [['Technique', tname] as [string, ReactNode]] : []),
    ...(p.cycles ? [['Breaths', `${p.cycles} cycles`] as [string, ReactNode]] : []),
    ...(p.lesson ? [['Lesson', <Link key="ls" to={`/path/${p.program}/${p.lesson}`}>{p.program === 'path' ? 'The Path' : p.program} · {p.lesson}</Link>] as [string, ReactNode]] : []),
    ...(p.hr_before || p.hr_after ? [['Heart rate', `${p.hr_before ?? '–'} → ${p.hr_after ?? '–'} bpm`] as [string, ReactNode]] : []),
    ...(p.tags ? [['Tags', <span key="t" className="chips">{p.tags.split(',').map(t => <span key={t} className="chip">{t.trim()}</span>)}</span>] as [string, ReactNode]] : []),
    ['Source', p.source === 'app' ? 'Timed in Pocket Still' : p.source === 'manual' ? 'Added by hand' : p.source === 'import' ? 'Imported' : `Logged by ${p.source}`],
    ['Pocket Health', p.health_synced_at ? <span key="h" className="ok">mindful minutes recorded {ago(p.health_synced_at)}</span> : p.health_error ? <span key="h" className="bad">{p.health_error}</span> : <span key="h" className="muted">not sent (off or not connected)</span>],
    ['Pocket Habits', p.habit_error ? <span key="hb" className="bad">{p.habit_error}</span> : p.habit_marked_at ? <span key="hb" className="muted">habit rules checked {ago(p.habit_marked_at)}</span> : <span key="hb" className="muted">waiting</span>],
  ]
  return (
    <div className="stack loose">
      <div className="practice-hero" style={{ ['--tile' as string]: color }}>
        <span className="tile-ico lg"><Icon name={KIND_ICON[p.kind] || 'sparkles'} size={22} /></span>
        <div className="grow"><h1>{p.title}</h1><div className="muted">{fmtDate(p.date)} · {fmtMin(practiceMin(p))}</div></div>
        <div className="row wrap">
          <button type="button" className="btn primary" onClick={again}><Icon name="play" size={14} />Again</button>
          <button type="button" className="btn" onClick={() => setEdit(true)}><Icon name="edit" size={14} />Edit</button>
          <button type="button" className="btn ghost danger-text" onClick={() => setDel(true)}><Icon name="trash" size={14} />Delete</button>
        </div>
      </div>
      {(p.calm_before != null || p.calm_after != null) && (
        <Card title="How you felt">
          <div className="calm-compare">
            <div><span className="stat-l">Before</span><span className={`calm-big c${p.calm_before ?? 0}`}>{p.calm_before ?? '–'}</span><span className="muted small">{p.calm_before ? CALM_LABEL[p.calm_before] : 'not rated'}</span></div>
            <Icon name="chevron-right" size={22} className="muted" />
            <div><span className="stat-l">After</span><span className={`calm-big c${p.calm_after ?? 0}`}>{p.calm_after ?? '–'}</span><span className="muted small">{p.calm_after ? CALM_LABEL[p.calm_after] : 'not rated'}</span></div>
            {p.calm_before != null && p.calm_after != null && <div className={`calm-delta ${p.calm_after > p.calm_before ? 'ok' : p.calm_after < p.calm_before ? 'bad' : 'muted'}`}>{p.calm_after - p.calm_before > 0 ? '+' : ''}{p.calm_after - p.calm_before}</div>}
          </div>
        </Card>
      )}
      {p.note && <Card title="Note"><p className="note-text">{p.note}</p></Card>}
      <Card title="Details"><dl className="set-dl">{rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl></Card>
      {edit && <PracticeModal practice={p} onClose={() => setEdit(false)} onSaved={np => { setEdit(false); setData(np); announceChange() }} />}
      {del && <Confirm title="Delete this practice?" message="It disappears from the journal and the statistics, and its mindful minutes are taken out of Pocket Health. Milestones stay unlocked." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={remove} />}
    </div>
  )
}
