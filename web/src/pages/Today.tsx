// Today: the day's goal ring, streak and level, what's next on the Path, a suggestion, quick starts and recent practice.
import { Link, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { lessonSession, sessionURL, type SessionSpec } from '../lib/session'
import type { HomeData, Practice, QuickItem } from '../lib/types'
import { fmtDate, fmtISOTime, weekday } from '../lib/format'
import { KIND_ICON, fmtMin, greeting, practiceMin, techniqueColor } from '../lib/still'
import { Card, Empty, Loading, useAsync } from '../components/ui'
import { Icon } from '../components/icons'
import { Ring } from '../components/Chart'
import { openCreate, useLibrary, useOnChange } from '../components/Layout'

export function quickSpec(q: QuickItem): SessionSpec {
  if (q.type === 'preset') return { mode: q.kind === 'breath' ? 'breath' : 'timer', preset: q.id }
  if (q.type === 'pattern') return { mode: 'breath', technique: q.key }
  return { mode: 'timer', technique: q.key, minutes: q.minutes }
}

export function CalmShift({ p }: { p: Pick<Practice, 'calm_before' | 'calm_after'> }) {
  if (p.calm_before == null && p.calm_after == null) return null
  const d = p.calm_before != null && p.calm_after != null ? p.calm_after - p.calm_before : 0
  return <span className={`calm-shift ${d > 0 ? 'up' : d < 0 ? 'down' : ''}`} title="How you felt before → after (1 tense … 5 calm)">{p.calm_before ?? '·'}<Icon name="chevron-right" size={11} />{p.calm_after ?? '·'}</span>
}

export function PracticeRow({ p, showDate = false }: { p: Practice; showDate?: boolean }) {
  const { library } = useLibrary()
  const color = techniqueColor(library, p.technique, p.kind)
  return (
    <Link to={`/journal/${p.id}`} className="list-row practice-row">
      <span className="list-ico" style={{ color, background: `color-mix(in srgb, ${color} 14%, transparent)` }}><Icon name={KIND_ICON[p.kind] || 'sparkles'} size={16} /></span>
      <span className="grow">
        <span className="pr-title">{p.title}{!p.completed && <span className="badge warn">stopped early</span>}{p.lesson && <span className="badge accent">lesson</span>}</span>
        <span className="muted small ellipsis">{showDate ? `${fmtDate(p.date, false)} · ` : ''}{fmtISOTime(p.started_at)}{p.note ? ` · ${p.note.slice(0, 80)}` : ''}</span>
      </span>
      <CalmShift p={p} />
      <span className="tnum pr-min">{fmtMin(practiceMin(p))}</span>
    </Link>
  )
}

export default function Today() {
  const { user } = useApp()
  const navigate = useNavigate()
  const { data: h, reload, error } = useAsync(() => api.get<HomeData>(`${V1}/home`), [])
  useOnChange(reload)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!h) return <Loading />
  const first = (user?.name || '').split(' ')[0]
  const fresh = h.lifetime.sessions === 0
  const lvl = h.lifetime
  const startLesson = () => h.next_lesson && navigate(sessionURL(lessonSession(h.next_lesson.practice, h.next_lesson.program, h.next_lesson.key)))
  return (
    <div className="stack loose today">
      <div className="today-head">
        <h1 className="home-title">{greeting(h.part_of_day)}{first && first !== 'Admin' ? `, ${first}` : ''}</h1>
        <p className="intention">“{h.intention}”</p>
      </div>

      {fresh ? (
        <Card className="welcome">
          <div className="welcome-body">
            <img src="/favicon.svg" alt="" width={56} height={56} />
            <div className="grow">
              <h2>Welcome to Pocket Still</h2>
              <p className="muted">Start with the Path: seven stages from your first three-minute breath to sitting still for twenty minutes. One short lesson a day is plenty. Or just breathe for a minute, right now.</p>
              <div className="row wrap mt">
                {h.next_lesson && <button type="button" className="btn primary" onClick={startLesson}><Icon name="play" size={14} />Begin “{h.next_lesson.title}”</button>}
                <button type="button" className="btn" onClick={() => navigate(sessionURL({ mode: 'breath', technique: 'sigh', minutes: 1 }))}><Icon name="wind" size={15} />One-minute reset</button>
                <Link to="/path" className="btn ghost">See the Path</Link>
              </div>
            </div>
          </div>
        </Card>
      ) : (
        <div className="today-kpis">
          <Card className="kpi-goal">
            <div className="row" style={{ gap: 16 }}>
              <Ring pct={h.today.goal_min ? (h.today.minutes / h.today.goal_min) * 100 : (h.today.minutes ? 100 : 0)} size={84} stroke={8}>
                <span className="ring-num"><b>{Math.round(h.today.minutes)}</b><small>min</small></span>
              </Ring>
              <div className="grow">
                <div className="stat-l">Today</div>
                <div className="kpi-v">{h.today.goal_min ? (h.today.minutes >= h.today.goal_min ? 'Goal reached' : `${Math.max(0, Math.ceil(h.today.goal_min - h.today.minutes))} min to your goal`) : `${h.today.sessions} practices`}</div>
                <div className="week-strip">
                  {h.week.map(d => <span key={d.date} className={`wk-day ${d.met ? 'met' : d.minutes > 0 ? 'some' : ''}`} title={`${fmtDate(d.date, false)}: ${fmtMin(d.minutes)}`}><i />{weekday(d.date).slice(0, 2)}</span>)}
                </div>
              </div>
            </div>
          </Card>
          <Card className="kpi-streak">
            <div className="stat-l">Streak</div>
            <div className="kpi-big"><Icon name="flame" size={22} className={h.streak ? 'flame-on' : 'muted'} />{h.streak}<small>{h.streak === 1 ? 'day' : 'days'}</small></div>
            <div className="muted small">Longest {h.longest_streak} · {h.lifetime.sessions} practices in all</div>
          </Card>
          <Card className="kpi-level">
            <div className="stat-l">Level</div>
            <div className="kpi-big"><Icon name="waves" size={20} />{lvl.level.name}</div>
            <div className="progress"><span style={{ width: `${lvl.level_pct}%` }} /></div>
            <div className="muted small">{lvl.next_level ? `${fmtMin(lvl.minutes)} · ${fmtMin(Math.max(0, lvl.next_level.minutes - lvl.minutes))} to ${lvl.next_level.name}` : `${fmtMin(lvl.minutes)} · the top level`}</div>
          </Card>
        </div>
      )}

      <div className="home-grid">
        {h.next_lesson && h.next_program && !fresh && (
          <Card title="Up next" sub={`${h.next_program.name} · ${h.next_program.kind === 'path' ? `Stage ${h.next_lesson.stage_idx + 1}: ${h.next_program.stage_name}` : `Lesson ${h.next_lesson.index + 1} of ${h.next_program.total}`}`}>
            <div className="next-lesson">
              <h3>{h.next_lesson.title}</h3>
              <p className="muted small">{h.next_lesson.teaching[0]}</p>
              <div className="row wrap mt">
                <button type="button" className="btn primary" onClick={startLesson}><Icon name="play" size={14} />Start · {h.next_lesson.practice.minutes} min</button>
                <Link className="btn ghost" to={`/path/${h.next_lesson.program}/${h.next_lesson.key}`}>Read the lesson</Link>
                <span className="muted small">{h.next_program.done}/{h.next_program.total} done</span>
              </div>
            </div>
          </Card>
        )}
        <Card title="Suggested for now" sub={h.suggestion.reason}>
          <div className="suggest" style={{ ['--tile' as string]: h.suggestion.color }}>
            <span className="tile-ico lg"><Icon name={h.suggestion.type === 'pattern' ? 'wind' : 'lotus'} size={18} /></span>
            <div className="grow"><b>{h.suggestion.name}</b><div className="muted small">{h.suggestion.minutes ? `${h.suggestion.minutes} min` : ''}</div></div>
            <button type="button" className="btn primary" onClick={() => navigate(sessionURL(h.suggestion.type === 'pattern' ? { mode: 'breath', technique: h.suggestion.key } : { mode: 'timer', technique: h.suggestion.key, minutes: h.suggestion.minutes }))}><Icon name="play" size={14} />Start</button>
          </div>
        </Card>
      </div>

      {h.quick.length > 0 && (
        <section className="stack">
          <h2 className="group-h">Your favourites</h2>
          <div className="quick-row">
            {h.quick.map(q => (
              <button key={q.key} type="button" className="quick-card" style={{ ['--tile' as string]: q.color || 'var(--accent)' }} onClick={() => navigate(sessionURL(quickSpec(q)))}>
                <span className="tile-ico"><Icon name={q.type === 'pattern' || q.kind === 'breath' ? 'wind' : 'lotus'} size={15} /></span>
                <span className="grow ellipsis">{q.name}</span>
                <Icon name="play" size={13} />
              </button>
            ))}
          </div>
        </section>
      )}

      {h.in_progress && h.in_progress.some(p => p.kind === 'course') && (
        <section className="stack">
          <h2 className="group-h">Courses in progress</h2>
          <div className="course-grid">
            {h.in_progress.filter(p => p.kind === 'course').map(p => (
              <Link key={p.key} to={`/path/${p.key}`} className="card course-card" style={{ ['--tile' as string]: p.color }}>
                <span className="tile-ico lg"><Icon name={p.icon} size={18} /></span>
                <span className="grow col"><b>{p.name}</b><span className="muted small">{p.next ? `Next: ${p.next.title}` : 'Done'}</span><span className="progress"><span style={{ width: `${p.percent}%` }} /></span></span>
              </Link>
            ))}
          </div>
        </section>
      )}

      <div className="home-grid">
        <Card title="Today" flush actions={<button type="button" className="btn ghost sm" onClick={() => openCreate('practice')}><Icon name="plus" size={14} />Add</button>}>
          {h.today_practices.length === 0 ? <p className="muted pad">Nothing yet today.</p> : <div className="list">{h.today_practices.map(p => <PracticeRow key={p.id} p={p} />)}</div>}
        </Card>
        <Card title="Recent" flush actions={<Link to="/journal" className="btn ghost sm">Journal</Link>}>
          {h.recent.length === 0 ? <Empty title="No practices yet" icon="journal" /> : <div className="list">{h.recent.slice(0, 6).map(p => <PracticeRow key={p.id} p={p} showDate />)}</div>}
        </Card>
      </div>

      {h.latest_milestones && h.latest_milestones.length > 0 && (
        <Card title="Latest milestones" actions={<Link to="/milestones" className="btn ghost sm">All</Link>}>
          <div className="ms-row">
            {h.latest_milestones.map(a => <div key={a.key} className="ms-chip"><span className="ms-ico"><Icon name={a.icon} size={16} /></span><span className="col"><b>{a.name}</b><span className="muted small">{fmtDate(a.unlocked_at.slice(0, 10), false)}</span></span></div>)}
          </div>
        </Card>
      )}
    </div>
  )
}
