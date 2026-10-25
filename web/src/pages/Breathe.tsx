// Breathe: the built-in patterns and your own, and one pattern's page.
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { sessionURL } from '../lib/session'
import { breathsPerMinute, cycleCount, fmtClock, patternCounts, totalSeconds } from '../lib/player'
import type { BreathConfig, Pattern, Preset } from '../lib/types'
import { fmtDate } from '../lib/format'
import { announceChange, useCrumb, useLibrary } from '../components/Layout'
import { BreathEditor } from '../components/BreathEditor'
import { PhaseDiagram } from '../components/BreathVisual'
import { Icon } from '../components/icons'
import { Card, Empty, Loading, PageHeader, Segmented, errMsg, useToast } from '../components/ui'

const GOOD: { value: string; label: string; icon: string }[] = [
  { value: 'all', label: 'All', icon: 'sparkles' }, { value: 'calm', label: 'Calm', icon: 'waves' }, { value: 'focus', label: 'Focus', icon: 'target' },
  { value: 'sleep', label: 'Sleep', icon: 'moon' }, { value: 'energy', label: 'Energy', icon: 'zap' }, { value: 'balance', label: 'Balance', icon: 'activity' },
]
const GOOD_NAME: Record<string, string> = { calm: 'Calm', focus: 'Focus', sleep: 'Sleep', energy: 'Energy', balance: 'Balance' }

function useStar() {
  const { library, reload } = useLibrary()
  const { toast } = useToast()
  return async (key: string) => {
    const on = !library?.favorites[key]
    try { await api.put(`${V1}/favorites/${key}`, { on }); reload(); announceChange(); toast(on ? 'On Today as a quick start' : 'Removed from Today') } catch (e) { toast(errMsg(e), 'err') }
  }
}

function lengthText(c: BreathConfig) {
  return c.cycles ? `${c.cycles} breaths` : `${c.minutes || 5} min`
}

export default function Breathe() {
  const { library, reload } = useLibrary()
  const navigate = useNavigate()
  const star = useStar()
  const [good, setGood] = useState('all')
  const [edit, setEdit] = useState<{ preset?: Preset } | null>(null)
  if (!library) return <Loading />
  const patterns = library.patterns.filter(p => good === 'all' || p.good_for === good)
  const mine = library.presets.filter(p => p.kind === 'breath')
  return (
    <div className="page breathe-page">
      <PageHeader title="Breathe" sub="Shape the breath to calm down, focus, wake up or fall asleep. Follow the circle; it does the counting."
        actions={<button type="button" className="btn" onClick={() => setEdit({})}><Icon name="plus" size={15} />New pattern</button>} />
      <div className="chips" role="radiogroup" aria-label="Good for">
        {GOOD.map(g => <button key={g.value} type="button" role="radio" aria-checked={good === g.value} className={`chip ${good === g.value ? 'active' : ''}`} onClick={() => setGood(g.value)}><Icon name={g.icon} size={14} />{g.label}</button>)}
      </div>
      <div className="pattern-grid">
        {patterns.map(p => {
          const use = library.uses[p.key]
          return (
            <article key={p.key} className="pattern-card" style={{ ['--pc' as string]: p.color }}>
              <Link to={`/breathe/${p.key}`} className="pc-main">
                <div className="pc-top">
                  <span className="pc-ico"><Icon name="wind" size={18} /></span>
                  <span className="pc-chip">{GOOD_NAME[p.good_for]}</span>
                  {p.level === 'intermediate' && <span className="pc-chip ghost">Intermediate</span>}
                </div>
                <h3>{p.name}</h3>
                <p className="muted small">{p.tagline}</p>
                <PhaseDiagram phases={p.config.phases} />
                <div className="pc-meta muted small"><span className="mono">{patternCounts(p.config.phases)}</span><span>· {lengthText(p.config)}</span>{use && <span className="pc-use">· {use.count}× done</span>}</div>
              </Link>
              <div className="pc-actions">
                <button type="button" className={`btn ghost icon-only sm star ${library.favorites[p.key] ? 'on' : ''}`} onClick={() => star(p.key)} aria-pressed={!!library.favorites[p.key]} title={library.favorites[p.key] ? 'Remove from Today' : 'Add to Today'}><Icon name="star" size={16} /></button>
                <button type="button" className="btn primary sm" onClick={() => navigate(sessionURL({ mode: 'breath', technique: p.key }))}><Icon name="play" size={13} />Start</button>
              </div>
            </article>
          )
        })}
      </div>

      <Card title="Your patterns" sub="Your own rhythms: any phases, any counts." actions={<button type="button" className="btn sm" onClick={() => setEdit({})}><Icon name="plus" size={14} />New</button>}>
        {mine.length === 0 ? <Empty title="No patterns of your own yet" icon="sliders">Start from scratch, or open a built-in pattern and choose “Make it my own”.</Empty> : (
          <div className="pattern-grid compact">
            {mine.map(p => {
              const c = p.config as BreathConfig
              const use = library.uses[`preset:${p.id}`]
              return (
                <article key={p.id} className="pattern-card mine">
                  <Link to={`/breathe/preset:${p.id}`} className="pc-main">
                    <div className="pc-top"><span className="pc-ico"><Icon name="sliders" size={16} /></span>{p.favorite && <span className="pc-chip">On Today</span>}</div>
                    <h3>{p.name}</h3>
                    <PhaseDiagram phases={c.phases} />
                    <div className="pc-meta muted small"><span className="mono">{patternCounts(c.phases)}</span><span>· {lengthText(c)}</span>{use && <span>· {use.count}×</span>}</div>
                  </Link>
                  <div className="pc-actions">
                    <button type="button" className="btn ghost icon-only sm" onClick={() => setEdit({ preset: p })} aria-label="Edit"><Icon name="edit" size={15} /></button>
                    <button type="button" className="btn primary sm" onClick={() => navigate(sessionURL({ mode: 'breath', preset: p.id }))}><Icon name="play" size={13} />Start</button>
                  </div>
                </article>
              )
            })}
          </div>
        )}
      </Card>
      {edit && <BreathEditor preset={edit.preset} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); reload(); announceChange() }} />}
    </div>
  )
}

export function PatternPage() {
  const { key = '' } = useParams()
  const { library, reload } = useLibrary()
  const navigate = useNavigate()
  const star = useStar()
  const [edit, setEdit] = useState<{ preset?: Preset; base?: { name: string; config: BreathConfig } } | null>(null)
  const presetID = key.startsWith('preset:') ? Number(key.slice(7)) : 0
  const preset = library?.presets.find(p => p.id === presetID && p.kind === 'breath')
  const pattern: Pattern | undefined = library?.patterns.find(p => p.key === key)
  const name = pattern?.name || preset?.name
  useCrumb(name)
  const config: BreathConfig | undefined = pattern?.config || (preset?.config as BreathConfig | undefined)
  const byCycles = !!config?.cycles
  const [len, setLen] = useState<number | null>(null)
  const value = len ?? (byCycles ? config?.cycles || 4 : config?.minutes || 5)
  const n = useMemo(() => (config ? cycleCount(config, byCycles ? undefined : value, byCycles ? value : undefined) : 0), [config, byCycles, value])
  if (!library) return <Loading />
  if (!config || !name) return <Empty title="Pattern not found" icon="wind"><Link to="/breathe">Back to Breathe</Link></Empty>
  const use = library.uses[key]
  const opts = [...new Set([...(byCycles ? [2, 4, 6, 8, 12] : [1, 2, 3, 5, 10, 15, 20]), value])].sort((a, b) => a - b)
  const start = () => navigate(sessionURL({ mode: 'breath', ...(pattern ? { technique: pattern.key } : { preset: presetID }), ...(byCycles ? { cycles: value } : { minutes: value }) }))
  return (
    <div className="page pattern-page" style={{ ['--pc' as string]: pattern?.color || 'var(--accent)' }}>
      <div className="pattern-hero">
        <div className="ph-text">
          <div className="pc-top">{pattern && <span className="pc-chip">{GOOD_NAME[pattern.good_for]}</span>}{pattern?.level === 'intermediate' && <span className="pc-chip ghost">Intermediate</span>}{preset && <span className="pc-chip">Your pattern</span>}</div>
          <h1>{name}</h1>
          {pattern && <p className="ph-tag">{pattern.tagline}</p>}
          <div className="ph-facts muted"><span className="mono">{patternCounts(config.phases)}</span><span>{breathsPerMinute(config.phases)} breaths a minute</span>{config.ease && <span>eases in</span>}</div>
        </div>
        <div className="ph-start">
          <Segmented size="sm" value={value} onChange={v => setLen(v)} options={opts.map(o => ({ value: o, label: byCycles ? `${o}×` : `${o}m` }))} label={byCycles ? 'Breaths' : 'Minutes'} />
          <div className="muted small">{n} breaths · {fmtClock(totalSeconds(config, n))}</div>
          <button type="button" className="btn primary lg" onClick={start}><Icon name="play" size={16} />Start</button>
          <div className="row wrap">
            {pattern && <button type="button" className={`btn ghost sm star ${library.favorites[pattern.key] ? 'on' : ''}`} onClick={() => star(pattern.key)}><Icon name="star" size={15} />{library.favorites[pattern.key] ? 'On Today' : 'Add to Today'}</button>}
            {pattern && <button type="button" className="btn ghost sm" onClick={() => setEdit({ base: { name: pattern.name, config: pattern.config } })}><Icon name="sliders" size={15} />Make it my own</button>}
            {preset && <button type="button" className="btn ghost sm" onClick={() => setEdit({ preset })}><Icon name="edit" size={15} />Edit</button>}
          </div>
        </div>
      </div>
      <Card title="One breath">
        <PhaseDiagram phases={config.phases} showLabels />
      </Card>
      {pattern && (
        <div className="two-col">
          <Card title="About">
            <p>{pattern.description}</p>
            {pattern.caution && <p className="caution"><Icon name="alert" size={15} />{pattern.caution}</p>}
          </Card>
          <Card title="How to do it">
            <ol className="how">{pattern.how.map((h, i) => <li key={i}>{h}</li>)}</ol>
          </Card>
        </div>
      )}
      <Card title="Your practice with it">
        {use ? (
          <div className="stats-row tight">
            <div className="stat"><span className="stat-l">Times</span><span className="stat-v">{use.count}</span></div>
            <div className="stat"><span className="stat-l">Minutes</span><span className="stat-v">{Math.round(use.minutes)}</span></div>
            <div className="stat"><span className="stat-l">Last</span><span className="stat-v">{fmtDate(use.last)}</span></div>
          </div>
        ) : <p className="muted">Not practised yet. Your sessions with it will show up here and in Statistics.</p>}
      </Card>
      {edit && <BreathEditor preset={edit.preset} base={edit.base} onClose={() => setEdit(null)} onSaved={p => { setEdit(null); reload(); announceChange(); if (!p && preset) navigate('/breathe'); else if (p && !preset) navigate(`/breathe/preset:${p.id}`) }} />}
    </div>
  )
}
