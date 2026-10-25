// Statistics: totals, streaks, the charts, how you felt, what helps, the year heatmap and Pocket Health.
import { type ReactNode, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import type { HealthInsights, HealthMetric, Stats as StatsT } from '../lib/types'
import { fmtDate } from '../lib/format'
import { KIND_COLOR, WEEKDAYS, fmtMin } from '../lib/still'
import { Card, CoreHint, Loading, PageHeader, Segmented, useAsync } from '../components/ui'
import { Icon } from '../components/icons'
import { Bars, HBars, Heatmap, LineChart, Scatter, type Pt } from '../components/Chart'
import { useOnChange } from '../components/Layout'

type Range = 'w' | 'm' | '3m' | 'y' | 'all'
const RANGES: { value: Range; label: string }[] = [{ value: 'w', label: 'Week' }, { value: 'm', label: 'Month' }, { value: '3m', label: '3 months' }, { value: 'y', label: 'Year' }, { value: 'all', label: 'All' }]
const PARTS: [keyof StatsT['parts_of_day'], string, string][] = [['morning', 'Morning', 'sunrise'], ['afternoon', 'Afternoon', 'sun'], ['evening', 'Evening', 'moon'], ['night', 'Night', 'bed']]

function Kpi({ label, value, sub, icon }: { label: string; value: ReactNode; sub?: ReactNode; icon?: string }) {
  return <div className="stat">{icon && <Icon name={icon} size={15} className="muted" />}<span className="stat-l">{label}</span><span className="stat-v tnum">{value}</span>{sub && <span className="stat-s small">{sub}</span>}</div>
}

const shiftClass = (v: number) => v > 0.05 ? 'ok' : v < -0.05 ? 'bad' : 'muted'
const signed = (v: number, d = 1) => `${v > 0 ? '+' : ''}${v.toFixed(d)}`

export default function Stats() {
  const { settings } = useApp()
  const [range, setRange] = useState<Range>(() => (localStorage.getItem('pst-stats-range') as Range) || 'm')
  const pick = (r: Range) => { setRange(r); localStorage.setItem('pst-stats-range', r) }
  const { data: s, reload, error } = useAsync(() => api.get<StatsT>(`${V1}/stats?range=${range}`), [range])
  useOnChange(reload)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  const head = <PageHeader title="Statistics" sub={s ? `${fmtDate(s.range.from)} – ${fmtDate(s.range.to)}` : undefined} actions={<div className="scroll-x"><Segmented value={range} onChange={pick} options={RANGES} size="sm" /></div>} />
  if (!s) return <div className="stack loose">{head}<Loading /></div>
  const t = s.totals
  const long = s.range.days > 120
  const series: Pt[] = long ? (s.range.days > 400 ? s.monthly.map(m => ({ x: m.month + '-01', y: m.minutes })) : s.weekly.map(w => ({ x: w.week, y: w.minutes }))) : s.daily.map(d => ({ x: d.date, y: d.minutes }))
  const goal = !long && s.goals.daily_goal_min > 0 ? s.goals.daily_goal_min : undefined
  const change = s.previous.change_pct
  const moodPts: Pt[] = s.mood.weekly.map(w => ({ x: w.week, y: w.avg_after }))
  return (
    <div className="stack loose">
      {head}
      <div className="stats-row">
        <Kpi label="Minutes" value={fmtMin(t.minutes)} sub={change !== undefined ? <span className={change >= 0 ? 'ok' : 'bad'}>{signed(change, 0)}% vs before</span> : `${t.avg_daily_min} min a day`} />
        <Kpi label="Practices" value={t.sessions} sub={`${t.completion_pct}% finished`} />
        <Kpi label="Days practised" value={`${t.days_practiced}/${s.range.days}`} sub={`${t.consistency_pct}% of days`} />
        <Kpi label="Average" value={fmtMin(t.avg_session_min)} sub={`longest ${fmtMin(t.longest_session_min)}`} />
        <Kpi label="Streak" value={<><Icon name="flame" size={16} className={s.streak.current ? 'flame-on' : 'muted'} /> {s.streak.current}</>} sub={`longest ${s.streak.longest}`} />
        <Kpi label="Goal streak" value={s.streak.goal_current} sub={s.goals.daily_goal_min ? `${s.goals.daily_goal_min}+ min days · best ${s.streak.goal_longest}` : 'no daily goal'} />
      </div>

      {s.insights.length > 0 && (
        <Card title="What stands out" sub="Plain patterns from your own practice.">
          <ul className="insights">{s.insights.map((x, i) => <li key={i}><Icon name="sparkles" size={14} />{x}</li>)}</ul>
        </Card>
      )}

      <Card title={long ? (s.range.days > 400 ? 'Minutes per month' : 'Minutes per week') : 'Minutes per day'} sub={goal ? `The dashed line is your daily goal; ${s.goals.days_met} of ${s.range.days} days reached it.` : undefined}>
        <LineChart points={series} kind="bar" goal={goal} goalLabel={goal ? `goal ${goal} min` : undefined} format={v => `${Math.round(v)}`} zero height={200} />
      </Card>

      <div className="home-grid">
        <Card title="Level" sub={s.lifetime.level.blurb}>
          <div className="level-box">
            <div className="kpi-big"><Icon name="waves" size={20} />{s.lifetime.level.name}</div>
            <div className="progress lg"><span style={{ width: `${s.lifetime.level_pct}%` }} /></div>
            <div className="muted small">{fmtMin(s.lifetime.minutes)} over {s.lifetime.sessions} practices since {s.lifetime.first_date ? fmtDate(s.lifetime.first_date) : 'today'}{s.lifetime.next_level ? ` · ${fmtMin(s.lifetime.minutes_to_next || 0)} to ${s.lifetime.next_level.name}` : ''}</div>
            <Link to="/milestones" className="btn ghost sm">Milestones</Link>
          </div>
        </Card>
        <Card title="This week" sub={`Goal: ${s.goals.weekly_goal_days} days of ${s.goals.daily_goal_min} min`}>
          <div className="row" style={{ gap: 18 }}>
            <div className="kpi-big">{s.goals.this_week_days}<small>/ {s.goals.weekly_goal_days || 7} days</small></div>
            <div className="muted">{fmtMin(s.goals.this_week_minutes)} since {fmtDate(s.goals.week_start, false)}</div>
          </div>
          <div className="progress lg mt"><span style={{ width: `${Math.min(100, (s.goals.this_week_days / (s.goals.weekly_goal_days || 7)) * 100)}%` }} /></div>
        </Card>
      </div>

      <Card title="The last year" sub="Every square is a day; the darker, the longer you practised.">
        <div className="scroll-x"><Heatmap days={s.heatmap} weekStart={settings?.week_start || 1} goal={s.goals.daily_goal_min} fmt={fmtMin} /></div>
      </Card>

      <Card title="How you felt" sub={s.mood.rated ? `${s.mood.rated} practices rated, 1 tense … 5 calm` : 'Rate how you feel before and after a practice to see what helps.'}>
        {s.mood.rated > 0 ? <>
          <div className="stats-row tight">
            <Kpi label="Before" value={s.mood.avg_before ? s.mood.avg_before.toFixed(1) : '–'} />
            <Kpi label="After" value={s.mood.avg_after.toFixed(1)} />
            <Kpi label="Shift" value={<span className={shiftClass(s.mood.avg_shift)}>{signed(s.mood.avg_shift)}</span>} />
            <Kpi label="Calmer after" value={`${s.mood.improved_pct}%`} sub={`of ${s.mood.with_before}`} />
          </div>
          {moodPts.length > 1 && <div className="mt"><LineChart points={moodPts} format={v => v.toFixed(1)} height={140} /></div>}
        </> : null}
      </Card>

      <Card title="What helps" sub="By technique: how often, how long, and how you felt before → after." flush>
        {s.by_technique.length === 0 ? <p className="muted pad">No practice in this period.</p> : (
          <table className="table compact tech-table">
            <thead><tr><th>Technique</th><th className="right">Practices</th><th className="right">Minutes</th><th className="right">Before → after</th><th className="right">Shift</th><th className="right">Finished</th></tr></thead>
            <tbody>{s.by_technique.map(x => (
              <tr key={x.key}>
                <td><span className="dot" style={{ background: KIND_COLOR[x.kind] || 'var(--accent)' }} />{x.name}</td>
                <td className="right tnum">{x.sessions}</td>
                <td className="right tnum">{Math.round(x.minutes)}</td>
                <td className="right tnum">{x.rated ? `${x.avg_before ? x.avg_before.toFixed(1) : '–'} → ${x.avg_after.toFixed(1)}` : <span className="muted">–</span>}</td>
                <td className={`right tnum ${x.rated ? shiftClass(x.avg_shift) : 'muted'}`}>{x.rated ? signed(x.avg_shift) : '–'}</td>
                <td className="right tnum">{x.completion_pct}%</td>
              </tr>
            ))}</tbody>
          </table>
        )}
      </Card>

      <div className="home-grid">
        <Card title="By kind">
          <HBars items={s.by_kind.map(k => ({ label: k.name, value: k.minutes, color: KIND_COLOR[k.kind], hint: `${k.sessions} practices` }))} format={v => fmtMin(v)} />
        </Card>
        <Card title="Length of a practice">
          <Bars items={s.lengths.map(l => ({ label: l.label.replace(' min', ''), value: l.sessions, hint: l.label }))} format={v => String(v)} height={130} />
        </Card>
      </div>

      <Card title="Time of day" sub="When your practices start.">
        <div className="parts-row">
          {PARTS.map(([k, label, icon]) => {
            const p = s.parts_of_day[k]
            return <div key={k} className="part-tile"><Icon name={icon} size={16} /><b>{label}</b><span className="tnum">{p.sessions} · {fmtMin(p.minutes)}</span>{p.avg_shift !== 0 && <span className={`small ${shiftClass(p.avg_shift)}`}>calm {signed(p.avg_shift)}</span>}</div>
          })}
        </div>
        <div className="mt"><Bars items={s.by_hour.map(h => ({ label: h.hour % 3 === 0 ? String(h.hour) : '', value: h.sessions, hint: `${h.hour}:00 · ${fmtMin(h.minutes)}` }))} format={v => String(v)} height={110} /></div>
      </Card>

      <Card title="Weekdays" sub="Average minutes per weekday, and how often you practised on it.">
        <Bars items={s.weekdays.map(w => ({ label: WEEKDAYS[w.weekday], value: w.avg_minutes, hint: `${w.practice_pct}% of ${WEEKDAYS[w.weekday]}s` }))} format={v => `${v}`} height={130} />
      </Card>

      <HealthCard />
      <ReflectCard range={range} />
    </div>
  )
}

function HealthMetricBox({ m }: { m: HealthMetric }) {
  const max = Math.max(m.practice_avg, m.rest_avg) || 1
  return (
    <div className="hm-metric">
      <div className="row"><b className="grow">{m.name}</b>{m.better !== undefined && <span className={`badge ${m.better ? 'ok' : 'warn'}`}>{m.diff_pct > 0 ? '+' : ''}{m.diff_pct}%</span>}</div>
      <div className="muted small">{m.when === 'next_night' ? 'The night after' : 'On the day'} · correlation with minutes {m.correlation.toFixed(2)}</div>
      <div className="cmp-bars">
        <div><span className="small">With practice</span><span className="cmp-track"><span style={{ width: `${(m.practice_avg / max) * 100}%` }} className="on" /></span><span className="tnum small">{m.practice_avg} {m.unit} <span className="muted">({m.n_practice})</span></span></div>
        <div><span className="small">Without</span><span className="cmp-track"><span style={{ width: `${(m.rest_avg / max) * 100}%` }} /></span><span className="tnum small">{m.rest_avg} {m.unit} <span className="muted">({m.n_rest})</span></span></div>
      </div>
      {m.points.length > 4 && <Scatter points={m.points.map(p => ({ x: p.minutes, y: p.value, label: `${p.date}: ${p.value} ${m.unit}, ${p.minutes} min` }))} xLabel="minutes" yLabel={m.unit} height={130} />}
    </div>
  )
}

function HealthCard() {
  const { settings } = useApp()
  const { data } = useAsync(() => api.get<HealthInsights>(`${V1}/stats/health`), [])
  if (!data) return null
  if (!data.ok) return (
    <Card title="Your body on practice days" sub="Heart rate variability, resting heart rate and sleep from Pocket Health, on days with practice against days without." className="muted-card">
      <div className="callout info"><Icon name="heart" size={15} /><div>{data.not_connected ? <>Connect Pocket Health under <Link to="/settings/apps">Settings → Connected apps</Link> to see this.</> : data.error}{!settings?.core_configured && <CoreHint />}</div></div>
    </Card>
  )
  return (
    <Card title="Your body on practice days" sub={data.note} actions={data.open_url ? <a className="btn ghost sm" href={data.open_url} target="_blank" rel="noreferrer"><Icon name="external" size={13} />Pocket Health</a> : undefined}>
      <div className="hm-grid">{(data.metrics || []).map(m => <HealthMetricBox key={m.key} m={m} />)}</div>
      {data.errors && data.errors.length > 0 && <p className="muted small mt">{data.errors.join(' · ')}</p>}
    </Card>
  )
}

function ReflectCard({ range }: { range: Range }) {
  const { data: st } = useAsync(() => api.get<{ ready: boolean; provider: string; detail: string }>(`${V1}/ai/status`), [])
  const [text, setText] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const run = async () => {
    setBusy(true); setErr('')
    try { const r = await api.post<{ ok: boolean; text?: string; error?: string }>(`${V1}/ai/reflect`, { range: range === 'all' ? 'y' : range }); if (r.ok) setText(r.text || ''); else setErr(r.error || 'failed') } catch (e) { setErr((e as Error).message) } finally { setBusy(false) }
  }
  return (
    <Card title="Reflection" sub="A few sentences on this period from your AI: what helped, what to notice, one thing to try next."
      actions={<button type="button" className="btn sm" disabled={!st?.ready || busy} onClick={run}><Icon name="sparkles" size={14} />{busy ? 'Writing…' : text ? 'Again' : 'Reflect'}</button>}>
      {st && !st.ready && <p className="muted small">No AI connected{st.detail ? ` (${st.detail})` : ''}. <Link to="/settings/ai">Connect one</Link>.</p>}
      {err && <p className="bad small">{err}</p>}
      {text && <div className="recap">{text.split('\n').filter(Boolean).map((l, i) => <p key={i}>{l}</p>)}</div>}
    </Card>
  )
}
