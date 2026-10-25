// The Path (seven stages as a trail) and the courses; a program's lessons; one lesson to read and practise.
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { lessonSession, sessionURL } from '../lib/session'
import type { Achievement, LessonView, ProgramView } from '../lib/types'
import { fmtDate } from '../lib/format'
import { KIND_ICON, fmtMin, techniqueName } from '../lib/still'
import { Card, Confirm, Loading, PageHeader, useAsync, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { announceChange, useCrumb, useLibrary, useOnChange } from '../components/Layout'

const lessonURL = (l: LessonView) => sessionURL(lessonSession(l.practice, l.program, l.key))

export default function Path() {
  const navigate = useNavigate()
  const { data: list, reload, error } = useAsync(() => api.get<ProgramView[]>(`${V1}/programs`), [])
  const { data: path, reload: reloadPath } = useAsync(() => api.get<ProgramView>(`${V1}/programs/path`), [])
  useOnChange(() => { reload(); reloadPath() })
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!list || !path) return <Loading />
  const courses = list.filter(p => p.kind === 'course')
  return (
    <div className="stack loose">
      <PageHeader title="The Path" sub={path.tagline} actions={path.next && <button type="button" className="btn primary" onClick={() => navigate(lessonURL(path.next!))}><Icon name="play" size={14} />{path.done ? 'Continue' : 'Begin'}</button>} />
      <Card>
        <div className="path-summary">
          <div className="grow">
            <p className="muted">{path.description}</p>
            <div className="progress lg mt"><span style={{ width: `${path.percent}%` }} /></div>
            <div className="muted small mt">{path.done} of {path.total} lessons{path.next ? ` · next: “${path.next.title}” in ${path.stage_name}` : path.completed_at ? ` · finished ${fmtDate(path.completed_at.slice(0, 10))}` : ''}</div>
          </div>
        </div>
      </Card>
      <ol className="trail">
        {path.stages!.map((st, i) => {
          const state = st.complete ? 'done' : st.locked ? 'locked' : 'open'
          const current = !st.complete && !st.locked
          return (
            <li key={st.key} className={`trail-stage ${state} ${current ? 'current' : ''}`}>
              <span className="trail-node">{st.complete ? <Icon name="check" size={16} strokeWidth={2.4} /> : st.locked ? <Icon name="lock" size={14} /> : i + 1}</span>
              <Link to={`/path/path#${st.key}`} className="card trail-card">
                <div className="row"><span className="stage-n">Stage {i + 1}</span><span className="spacer" /><span className="muted small tnum">{st.done}/{st.total} · {fmtMin(st.minutes)}</span></div>
                <h3>{st.name}</h3>
                <p className="muted small">{st.tagline}</p>
                <div className="progress"><span style={{ width: `${(st.done / st.total) * 100}%` }} /></div>
              </Link>
            </li>
          )
        })}
      </ol>
      <h2 className="group-h">Courses</h2>
      <div className="course-grid">
        {courses.map(p => (
          <Link key={p.key} to={`/path/${p.key}`} className="card course-card" style={{ ['--tile' as string]: p.color }}>
            <span className="tile-ico lg"><Icon name={p.icon} size={18} /></span>
            <span className="grow col">
              <b>{p.name}</b>
              <span className="muted small">{p.tagline}</span>
              <span className="progress"><span style={{ width: `${p.percent}%` }} /></span>
              <span className="muted xsmall">{p.completed_at ? 'Finished' : p.done ? `${p.done}/${p.total} done` : `${p.total} lessons`}</span>
            </span>
          </Link>
        ))}
      </div>
    </div>
  )
}

export function ProgramPage() {
  const { program = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { library } = useLibrary()
  const { data: p, setData, error } = useAsync(() => api.get<ProgramView>(`${V1}/programs/${program}`), [program])
  const [reset, setReset] = useState(false)
  useCrumb(p?.kind === 'course' ? p.name : p ? 'All stages' : '')
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!p) return <Loading />
  const doReset = async () => { setReset(false); try { setData(await api.post<ProgramView>(`${V1}/programs/${program}/reset`)); announceChange(); toast('Progress reset') } catch (e) { toast((e as Error).message, 'err') } }
  return (
    <div className="stack loose">
      <PageHeader title={p.name} sub={p.tagline} actions={<>
        {p.next && <button type="button" className="btn primary" onClick={() => navigate(lessonURL(p.next!))}><Icon name="play" size={14} />{p.done ? 'Continue' : 'Begin'}</button>}
        {p.done > 0 && <button type="button" className="btn ghost" onClick={() => setReset(true)}><Icon name="undo" size={14} />Start over</button>}
      </>} />
      <Card>
        <p className="muted">{p.description}</p>
        <div className="progress lg mt" style={{ ['--tile' as string]: p.color }}><span style={{ width: `${p.percent}%` }} /></div>
        <div className="muted small mt">{p.done} of {p.total} lessons{p.started_at ? ` · started ${fmtDate(p.started_at.slice(0, 10))}` : ''}{p.completed_at ? ` · finished ${fmtDate(p.completed_at.slice(0, 10))}` : ''}</div>
      </Card>
      {p.stages!.map((st, i) => (
        <Card key={st.key} title={p.kind === 'path' ? <span id={st.key}>Stage {i + 1} · {st.name}</span> : st.name} sub={st.tagline} flush
          actions={<span className={`badge ${st.complete ? 'ok' : st.locked ? '' : 'accent'}`}>{st.complete ? 'done' : st.locked ? 'locked' : `${st.done}/${st.total}`}</span>}>
          <div className="list">
            {st.lessons.map(l => (
              <Link key={l.key} to={`/path/${p.key}/${l.key}`} className={`list-row lesson-row ${l.done ? 'done' : ''} ${l.current ? 'current' : ''} ${l.locked ? 'locked' : ''}`}>
                <span className="lesson-mark">{l.done ? <Icon name="check" size={14} strokeWidth={2.4} /> : l.locked ? <Icon name="lock" size={13} /> : l.index + 1}</span>
                <span className="grow col"><b>{l.title}</b><span className="muted small">{techniqueName(library, l.practice.technique) || l.practice.technique} · {l.practice.minutes} min{l.done_at ? ` · done ${fmtDate(l.done_at.slice(0, 10), false)}` : ''}</span></span>
                <Icon name={KIND_ICON[l.practice.kind] || 'lotus'} size={15} className="muted" />
                {l.current && <span className="badge accent">next</span>}
              </Link>
            ))}
          </div>
        </Card>
      ))}
      {reset && <Confirm title={`Start ${p.name} over?`} message="Your progress through the lessons is forgotten. Your practices stay in the journal and your milestones stay unlocked." confirmLabel="Start over" onCancel={() => setReset(false)} onConfirm={doReset} />}
    </div>
  )
}

export function LessonPage() {
  const { program = '', lesson = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { library } = useLibrary()
  const { data: p, reload, error } = useAsync(() => api.get<ProgramView>(`${V1}/programs/${program}`), [program])
  const [busy, setBusy] = useState(false)
  const all = p?.stages?.flatMap(s => s.lessons) || []
  const i = all.findIndex(l => l.key === lesson)
  const l = all[i]
  useCrumb(l?.title)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!p) return <Loading />
  if (!l) return <div className="callout danger"><Icon name="alert" size={15} />No such lesson.</div>
  const stage = p.stages![l.stage_idx]
  const prev = all[i - 1], next = all[i + 1]
  const tech = library?.patterns.find(x => x.key === l.practice.technique) || library?.styles.find(x => x.key === l.practice.technique)
  const markDone = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ unlocked: Achievement[] }>(`${V1}/programs/${program}/lessons/${lesson}/complete`)
      toast(r.unlocked?.length ? `Lesson done · new milestone: ${r.unlocked.map(a => a.name).join(', ')}` : 'Lesson done')
      announceChange(); reload()
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <div className="stack loose lesson-page">
      <div className="lesson-kicker muted small"><Link to={`/path/${p.key}`}>{p.name}</Link>{p.kind === 'path' && <> · Stage {l.stage_idx + 1}: {stage.name}</>} · Lesson {l.index + 1} of {p.total}</div>
      <h1 className="lesson-title">{l.title}{l.done && <span className="badge ok">done</span>}</h1>
      <article className="lesson-text">
        {l.teaching.map((t, k) => <p key={k}>{t}</p>)}
        {l.tip && <div className="callout info"><Icon name="sparkles" size={15} /><div><b>Tip</b> · {l.tip}</div></div>}
      </article>
      <Card>
        <div className="lesson-practice" style={{ ['--tile' as string]: tech?.color || 'var(--accent)' }}>
          <span className="tile-ico lg"><Icon name={KIND_ICON[l.practice.kind] || 'lotus'} size={20} /></span>
          <div className="grow col">
            <b>{tech?.name || l.practice.technique} · {l.practice.minutes} min</b>
            <span className="muted small">{tech?.tagline}</span>
          </div>
          <button type="button" className="btn primary" disabled={l.locked} onClick={() => navigate(lessonURL(l))}><Icon name="play" size={14} />{l.done ? 'Practise again' : 'Begin practice'}</button>
        </div>
        {l.locked && <p className="muted small mt"><Icon name="lock" size={12} /> Finish the lessons before this one first.</p>}
        {!l.done && !l.locked && <button type="button" className="link-btn small mt" disabled={busy} onClick={markDone}>Practised without the timer? Mark as done</button>}
      </Card>
      <div className="row lesson-nav">
        {prev ? <Link className="btn ghost" to={`/path/${p.key}/${prev.key}`}><Icon name="chevron-left" size={15} />{prev.title}</Link> : <span />}
        <span className="spacer" />
        {next && <Link className="btn ghost" to={`/path/${p.key}/${next.key}`}>{next.title}<Icon name="chevron-right" size={15} /></Link>}
      </div>
    </div>
  )
}
