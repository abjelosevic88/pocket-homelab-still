// The full-screen player: get ready (length, sound, how you feel), breathe or
// sit along with the visual, bells and cues, then say how you feel and save.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { parseSession } from '../lib/session'
import { AMBIENT_NAMES, bell, isMuted, pauseAll, ping, setMuted, setVolume, speak, startAmbient, stopAmbient, stopSpeech, swell, unlock, vibrate } from '../lib/audio'
import { type BreathMoment, PHASE_LABEL, PHASE_SPOKEN, cycleCount, fmtClock, momentAt, totalSeconds } from '../lib/player'
import type { BreathConfig, Cue, Library, MusicPlaylist, PracticeResult, ProgramView, Style, TimerConfig } from '../lib/types'
import { BreathVisual } from '../components/BreathVisual'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Loading, errMsg } from '../components/ui'

const CALM = [
  { v: 1, label: 'Tense' }, { v: 2, label: 'Uneasy' }, { v: 3, label: 'Okay' }, { v: 4, label: 'Settled' }, { v: 5, label: 'Calm' },
]

const ACH_ICON: Record<string, string> = { 'calendar-check': 'calendar-check' }

interface Resolved {
  mode: 'breath' | 'timer'
  name: string
  technique: string // pattern / style key or preset:<id>
  kind: string
  color: string
  breath?: BreathConfig
  timer?: TimerConfig
  style?: Style
  byCycles: boolean
}

function CalmScale({ value, onChange, label }: { value: number | null; onChange: (v: number | null) => void; label: string }) {
  return (
    <div className="calm-scale" role="radiogroup" aria-label={label}>
      {CALM.map(c => (
        <button key={c.v} type="button" role="radio" aria-checked={value === c.v} className={`calm-btn calm-${c.v} ${value === c.v ? 'on' : ''}`} onClick={() => onChange(value === c.v ? null : c.v)}>
          <span className="calm-dot" /><span>{c.label}</span>
        </button>
      ))}
    </div>
  )
}

function uuid(): string {
  try { return crypto.randomUUID() } catch { return `${Date.now()}-${Math.random().toString(36).slice(2)}` }
}

export default function Session() {
  const [sp] = useSearchParams()
  const spec = useMemo(() => parseSession(sp), [sp])
  const cfgParam = sp.get('cfg')
  const { settings } = useApp()
  const navigate = useNavigate()
  const [lib, setLib] = useState<Library | null>(null)
  const [err, setErr] = useState('')
  const [lessonTitle, setLessonTitle] = useState('')
  const [lists, setLists] = useState<MusicPlaylist[]>([])

  useEffect(() => {
    api.get<Library>(`${V1}/library`).then(setLib).catch(e => setErr(errMsg(e)))
    api.get<{ ok: boolean; playlists?: MusicPlaylist[] }>(`${V1}/music/playlists`).then(r => setLists(r.ok ? r.playlists || [] : [])).catch(() => {})
  }, [])
  useEffect(() => {
    if (!spec.program || !spec.lesson) return
    api.get<ProgramView>(`${V1}/programs/${spec.program}`).then(p => {
      for (const st of p.stages || []) for (const l of st.lessons) if (l.key === spec.lesson) setLessonTitle(`${p.name} · ${st.name}: ${l.title}`)
    }).catch(() => {})
  }, [spec.program, spec.lesson])

  const resolved = useMemo<Resolved | null>(() => {
    if (!lib || !settings) return null
    const preset = spec.preset ? lib.presets.find(p => p.id === spec.preset) : undefined
    if (spec.mode === 'breath') {
      const pat = lib.patterns.find(p => p.key === spec.technique)
      const cfg = (preset?.kind === 'breath' ? (preset.config as BreathConfig) : pat?.config) || lib.patterns[0]?.config
      if (!cfg) return null
      const byCycles = spec.cycles ? true : spec.minutes ? false : !!cfg.cycles
      return { mode: 'breath', name: preset?.name || pat?.name || 'Breathing', technique: preset ? `preset:${preset.id}` : pat?.key || '', kind: 'breath', color: pat?.color || '#38bdf8', breath: cfg, byCycles }
    }
    let timer: TimerConfig | undefined = preset?.kind === 'timer' ? (preset.config as TimerConfig) : undefined
    if (!timer && cfgParam) { try { timer = JSON.parse(cfgParam) as TimerConfig } catch { /* ignore */ } }
    const styleKey = timer?.style || spec.technique || 'silent'
    const style = lib.styles.find(s => s.key === styleKey) || lib.styles.find(s => s.key === 'silent')
    if (!timer) {
      timer = { minutes: spec.minutes ?? style?.minutes ?? settings.default_sit_min, warmup_s: 10, interval_min: 0, start_bell: settings.bell, interval_bell: 'chime', end_bell: settings.bell,
        ambient: settings.ambient || 'none', style: style?.key || 'silent', guidance: style?.cues?.length ? 'full' : 'off' }
    } else if (spec.minutes !== undefined && !preset) timer = { ...timer, minutes: spec.minutes }
    return { mode: 'timer', name: preset?.name || style?.name || 'Meditation', technique: preset ? `preset:${preset.id}` : style?.key || 'silent', kind: style?.kind || 'meditation', color: style?.color || '#0f766e', timer, style, byCycles: false }
  }, [lib, settings, spec, cfgParam])

  if (err) return <div className="player theme-minimal"><div className="pl-center"><p>{err}</p><Link to="/" className="btn">Back</Link></div></div>
  if (!resolved || !settings) return <div className="player theme-dusk"><div className="pl-center"><Loading /></div></div>
  return <Player r={resolved} spec={spec} lessonTitle={lessonTitle} lists={lists} onExit={to => navigate(to)} />
}

type Stage = 'intro' | 'run' | 'finish' | 'result'

function Player({ r, spec, lessonTitle, lists, onExit }: { r: Resolved; spec: ReturnType<typeof parseSession>; lessonTitle: string; lists: MusicPlaylist[]; onExit: (to: string) => void }) {
  const { settings } = useApp()
  const st = settings!
  const reduced = useMemo(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches, [])
  const [stage, setStage] = useState<Stage>('intro')
  // length: minutes, or breaths for patterns counted in breaths
  const initLen = r.mode === 'breath' ? (r.byCycles ? spec.cycles || r.breath!.cycles || 4 : spec.minutes || r.breath!.minutes || 5) : r.timer!.minutes
  const [len, setLen] = useState<number>(initLen)
  const [ambient, setAmbient] = useState<string>(r.mode === 'timer' ? r.timer!.ambient || 'none' : r.breath!.ambient || st.ambient || 'none')
  const [calmBefore, setCalmBefore] = useState<number | null>(null)
  const [calmAfter, setCalmAfter] = useState<number | null>(null)
  const [note, setNote] = useState('')
  const [muted, setMutedState] = useState(isMuted())
  const [paused, setPaused] = useState(false)
  const [confirmEnd, setConfirmEnd] = useState(false)
  const [t, setT] = useState(0) // seconds of practice (breath) / since Begin (timer, incl. warm-up)
  const [cue, setCue] = useState<string>('')
  const [saving, setSaving] = useState(false)
  const [saveErr, setSaveErr] = useState('')
  const [result, setResult] = useState<PracticeResult | null>(null)
  const [outcome, setOutcome] = useState<{ duration: number; completed: boolean; cycles: number } | null>(null)

  // breath geometry
  const n = r.mode === 'breath' ? cycleCount(r.breath!, r.byCycles ? undefined : len, r.byCycles ? len : undefined) : 0
  const breathTotal = r.mode === 'breath' ? totalSeconds(r.breath!, n) : 0
  const timer = r.mode === 'timer' ? { ...r.timer!, minutes: len } : null
  const warm = timer ? timer.warmup_s : 3 // breathing gets a three-second "get ready"
  const planned = r.mode === 'breath' ? breathTotal : (timer!.minutes || 0) * 60

  // the clock
  const startWall = useRef<string>('')
  const extID = useRef<string>('')
  const base = useRef(0) // performance.now() at Begin, shifted by pauses
  const pausedAt = useRef(0)
  const lastKey = useRef('')
  const evt = useRef({ started: false, interval: 0, cue: -1, ended: false })
  const wake = useRef<WakeLockSentinel | null>(null)

  const cues: Cue[] = useMemo(() => {
    if (!timer || !r.style?.cues || timer.guidance === 'off' || !timer.guidance) return []
    const list = r.style.cues.filter(c => timer.guidance === 'full' || c.key)
    return timer.minutes ? list : list.filter(c => c.at === 0)
  }, [timer?.guidance, timer?.minutes, r.style]) // eslint-disable-line react-hooks/exhaustive-deps

  const lockScreen = useCallback(async () => {
    if (!st.keep_awake || !('wakeLock' in navigator)) return
    try { wake.current = await navigator.wakeLock.request('screen') } catch { /* not allowed */ }
  }, [st.keep_awake])
  useEffect(() => {
    const h = () => { if (document.visibilityState === 'visible' && stage === 'run' && !paused) lockScreen() }
    document.addEventListener('visibilitychange', h)
    return () => document.removeEventListener('visibilitychange', h)
  }, [stage, paused, lockScreen])
  useEffect(() => () => { stopAmbient(0.5); stopSpeech(); wake.current?.release().catch(() => {}); if (document.fullscreenElement) document.exitFullscreen().catch(() => {}) }, [])

  const begin = async () => {
    await unlock()
    setVolume(st.volume ?? 0.8)
    startAmbient(ambient)
    lockScreen()
    startWall.current = new Date().toISOString()
    extID.current = uuid()
    base.current = performance.now()
    evt.current = { started: false, interval: 0, cue: -1, ended: false }
    lastKey.current = ''
    setT(0)
    setStage('run')
    if (r.mode === 'breath' && st.voice) speak('Get ready', st.voice_name)
  }

  const practisedSeconds = (tt: number) => Math.max(0, tt - warm)

  const finish = useCallback((tt: number, natural: boolean) => {
    if (evt.current.ended) return
    evt.current.ended = true
    const dur = practisedSeconds(tt)
    let completed = natural
    if (!natural) completed = planned > 0 ? dur >= planned * 0.95 : true
    const cyc = r.mode === 'breath' ? (natural ? n : momentAt(r.breath!, n, dur).cycle) : 0
    setOutcome({ duration: Math.round(planned > 0 ? Math.min(dur, planned + 1) : dur), completed, cycles: cyc })
    stopAmbient(3)
    stopSpeech()
    wake.current?.release().catch(() => {})
    wake.current = null
    vibrate(st.haptics ? [80, 60, 80] : 0)
    setStage('finish')
  }, [planned, n, r.mode, r.breath, st.haptics]) // eslint-disable-line react-hooks/exhaustive-deps

  // one step of the clock: runs every frame while visible and twice a second in the background
  const step = useCallback(() => {
    if (stage !== 'run' || paused || evt.current.ended) return
    const tt = (performance.now() - base.current) / 1000
    setT(tt)
    const p = tt - warm
    if (r.mode === 'breath') {
      if (p < 0) return
      const m = momentAt(r.breath!, n, p)
      if (m.done) { finish(tt, true); return }
      const key = `${m.cycle}:${m.index}`
      if (key !== lastKey.current) {
        lastKey.current = key
        if (st.breath_sound === 'soft') swell(m.phase.type, m.phaseDur)
        else if (st.breath_sound === 'bell') ping(m.phase.type === 'in' || m.phase.type === 'in2')
        if (st.haptics) vibrate(m.phase.type === 'in' || m.phase.type === 'in2' ? 40 : m.phase.type === 'out' ? [20, 40, 20] : 15)
        if (st.voice) speak(m.phase.label || PHASE_SPOKEN[m.phase.type], st.voice_name)
      }
      return
    }
    const tm = timer!
    if (p >= 0 && !evt.current.started) {
      evt.current.started = true
      bell(tm.start_bell)
      if (st.haptics) vibrate(60)
    }
    if (p < 0) return
    if (tm.interval_min > 0) {
      const k = Math.floor(p / (tm.interval_min * 60))
      if (k > evt.current.interval && (!tm.minutes || p < tm.minutes * 60 - 5)) { evt.current.interval = k; bell(tm.interval_bell, 0.6) }
    }
    const total = (tm.minutes || 0) * 60
    for (let i = evt.current.cue + 1; i < cues.length; i++) {
      const at = cues[i].at * total + (cues[i].at === 0 ? 4 : 0)
      if (p >= at) {
        evt.current.cue = i
        setCue(cues[i].text)
        speak(cues[i].text, st.voice_name)
      } else break
    }
    if (total > 0 && p >= total) {
      bell(tm.end_bell)
      finish(tt, true)
    }
  }, [stage, paused, warm, r.mode, r.breath, n, timer, cues, st, finish])

  useEffect(() => {
    if (stage !== 'run' || paused) return
    let raf = 0
    const loop = () => { step(); raf = requestAnimationFrame(loop) }
    raf = requestAnimationFrame(loop)
    const iv = window.setInterval(step, 500)
    return () => { cancelAnimationFrame(raf); window.clearInterval(iv) }
  }, [stage, paused, step])

  const togglePause = useCallback(() => {
    if (stage !== 'run') return
    setPaused(pz => {
      if (!pz) { pausedAt.current = performance.now(); pauseAll(true); if ('speechSynthesis' in window) speechSynthesis.pause() }
      else { base.current += performance.now() - pausedAt.current; pauseAll(false); if ('speechSynthesis' in window) speechSynthesis.resume(); lastKey.current = '' }
      return !pz
    })
  }, [stage])

  const progress = planned > 0 ? Math.min(1, practisedSeconds(t) / planned) : 0
  const askEnd = useCallback(() => {
    if (stage !== 'run') return
    if (planned > 0 && progress < 0.8) { if (!paused) togglePause(); setConfirmEnd(true) }
    else finish(t, false)
  }, [stage, planned, progress, paused, togglePause, finish, t])

  const toggleMute = () => { const m = !muted; setMuted(m); setMutedState(m); if (m) stopSpeech() }
  const fullscreen = () => { if (document.fullscreenElement) document.exitFullscreen().catch(() => {}); else document.documentElement.requestFullscreen?.().catch(() => {}) }

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const tg = e.target as HTMLElement
      if (tg.closest('input, textarea, select, [contenteditable="true"]')) return
      if (stage === 'run') {
        if (e.key === ' ') { e.preventDefault(); if (confirmEnd) return; togglePause() }
        else if (e.key === 'Escape') { e.preventDefault(); if (confirmEnd) { setConfirmEnd(false); togglePause() } else askEnd() }
        else if (e.key === 'f') fullscreen()
        else if (e.key === 'm') toggleMute()
      } else if (stage === 'intro' && e.key === 'Enter') { e.preventDefault(); begin() }
      else if (stage === 'intro' && e.key === 'Escape') onExit(back())
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const back = () => (spec.program && spec.lesson ? `/path/${spec.program}/${spec.lesson}` : r.mode === 'breath' ? '/breathe' : '/meditate')

  const save = async () => {
    if (!outcome) return
    setSaving(true)
    setSaveErr('')
    try {
      const res = await api.post<PracticeResult>(`${V1}/practices`, {
        kind: r.kind, technique: r.technique, title: r.technique.startsWith('preset:') ? r.name : undefined, started_at: startWall.current,
        duration_s: outcome.duration, planned_s: Math.round(planned), completed: outcome.completed, cycles: outcome.cycles,
        calm_before: calmBefore ?? undefined, calm_after: calmAfter ?? undefined, note: note.trim(),
        program: spec.program, lesson: spec.lesson, external_id: extID.current,
      })
      setResult(res)
      setStage('result')
      window.dispatchEvent(new CustomEvent('st:changed'))
      if (res.unlocked?.length) { bell('chime', 0.7); vibrate(st.haptics ? [40, 40, 40, 40, 120] : 0) }
    } catch (e) { setSaveErr(errMsg(e)) } finally { setSaving(false) }
  }

  const theme = `theme-${st.player_theme || 'dusk'} ${reduced ? 'reduced' : ''}`
  const style = { ['--pc' as string]: r.color }

  // ---------- intro ----------
  if (stage === 'intro') {
    const breathOpts = r.byCycles ? [2, 4, 6, 8, 12] : [1, 2, 3, 5, 10, 15, 20]
    const stepLen = (d: number) => setLen(v => {
      if (r.mode === 'breath' && r.byCycles) return Math.max(1, Math.min(100, v + d))
      if (r.mode === 'timer' && v === 0 && d < 0) return 0
      const steps = r.mode === 'breath' ? breathOpts : [0, 1, 2, 3, 5, 10, 15, 20, 25, 30, 40, 45, 60, 90]
      const i = steps.findIndex(x => x >= v)
      const j = d > 0 ? (steps[i] === v ? i + 1 : i) : (i === -1 ? steps.length - 1 : i - 1)
      return steps[Math.max(0, Math.min(steps.length - 1, j))]
    })
    const lenText = r.mode === 'breath' ? (r.byCycles ? `${len} breaths` : `${len} min`) : len ? `${len} min` : 'Open-ended'
    return (
      <div className={`player ${theme}`} style={style}>
        <div className="pl-top"><button type="button" className="pl-icon" onClick={() => onExit(back())} aria-label="Close"><Icon name="x" size={22} /></button></div>
        <div className="pl-intro">
          {lessonTitle && <div className="pl-lesson"><Icon name="mountain" size={14} />{lessonTitle}</div>}
          <h1 className="pl-name">{r.name}</h1>
          <p className="pl-sub">{r.mode === 'breath' ? `${n} breaths · ${fmtClock(breathTotal)}` : r.style?.tagline}</p>
          <div className="pl-len">
            <button type="button" className="pl-round" onClick={() => stepLen(-1)} aria-label="Shorter"><Icon name="minus" size={20} /></button>
            <div className="pl-len-v">{lenText}</div>
            <button type="button" className="pl-round" onClick={() => stepLen(1)} aria-label="Longer"><Icon name="plus" size={20} /></button>
          </div>
          <div className="pl-amb">
            <Icon name="waves" size={16} />
            <Select value={ambient} onChange={setAmbient} ariaLabel="Background sound"
              options={[...Object.entries(AMBIENT_NAMES).map(([value, label]) => ({ value, label, group: 'Sounds' })), ...lists.map(l => ({ value: `music:${l.id}`, label: l.name, group: 'Your music' }))]} />
          </div>
          {st.ask_calm && (
            <div className="pl-ask">
              <p>How do you feel right now?</p>
              <CalmScale value={calmBefore} onChange={setCalmBefore} label="How you feel before" />
            </div>
          )}
          <button type="button" className="pl-begin" onClick={begin} autoFocus><Icon name="play" size={20} />Begin</button>
          {r.mode === 'breath' && <p className="pl-hint">Follow the circle: it grows as you breathe in and shrinks as you breathe out.</p>}
        </div>
      </div>
    )
  }

  // ---------- running ----------
  if (stage === 'run') {
    const p = t - warm
    const getReady = p < 0
    let m: BreathMoment | null = null
    if (r.mode === 'breath' && !getReady) m = momentAt(r.breath!, n, p)
    const remaining = planned > 0 ? planned - Math.max(0, p) : Math.max(0, p)
    return (
      <div className={`player running ${theme} ${paused ? 'is-paused' : ''}`} style={style}>
        <div className="pl-top">
          <button type="button" className="pl-icon" onClick={askEnd} aria-label="End"><Icon name="x" size={22} /></button>
          <span className="pl-top-title">{r.name}</span>
          <button type="button" className="pl-icon" onClick={toggleMute} aria-label={muted ? 'Sound on' : 'Mute'}><Icon name={muted ? 'volume-x' : 'volume'} size={20} /></button>
          <button type="button" className="pl-icon hide-phone" onClick={fullscreen} aria-label="Full screen"><Icon name="maximize" size={20} /></button>
        </div>
        <div className="pl-stage" onClick={e => { if (e.target === e.currentTarget) togglePause() }}>
          {r.mode === 'breath' ? (
            getReady ? (
              <div className="pl-ready"><div className="pl-ready-n">{Math.ceil(-p)}</div><div className="pl-phase">Get ready</div></div>
            ) : m && (
              <>
                <BreathVisual visual={st.breath_visual || 'orb'} m={m} phases={r.breath!.phases} />
                <div className="pl-phase" aria-live="polite">{m.phase.label || PHASE_LABEL[m.phase.type]}</div>
                <div className="pl-count">{Math.max(1, Math.ceil(m.phaseLeft))}</div>
              </>
            )
          ) : (
            <TimerRing p={p} total={planned} warm={warm} />
          )}
          {cue && r.mode === 'timer' && p >= 0 && <p className="pl-cue" key={cue}>{cue}</p>}
        </div>
        <div className="pl-bottom">
          <div className="pl-meta">
            {r.mode === 'breath' && m && <span>Breath {Math.min(n, m.cycle + 1)} of {n}</span>}
            <span>{planned > 0 ? `${fmtClock(remaining)} left` : `${fmtClock(remaining)}`}</span>
          </div>
          {planned > 0 && <div className="pl-progress"><i style={{ width: `${progress * 100}%` }} /></div>}
          <div className="pl-controls">
            <button type="button" className="pl-round lg" onClick={togglePause} aria-label={paused ? 'Go on' : 'Pause'}><Icon name={paused ? 'play' : 'pause'} size={26} /></button>
          </div>
          {paused && <p className="pl-hint">Paused. Tap to go on, or end the practice.</p>}
        </div>
        {confirmEnd && (
          <div className="pl-sheet" role="dialog" aria-label="End now?">
            <h2>End now?</h2>
            <p>It's saved as stopped early. Every minute still counts.</p>
            <div className="pl-sheet-btns">
              <button type="button" className="pl-btn" onClick={() => { setConfirmEnd(false); togglePause() }}>Keep going</button>
              <button type="button" className="pl-btn primary" onClick={() => { setConfirmEnd(false); finish((pausedAt.current - base.current) / 1000, false) }}>End</button>
            </div>
          </div>
        )}
      </div>
    )
  }

  // ---------- finish: how do you feel now ----------
  if (stage === 'finish' && outcome) {
    return (
      <div className={`player ${theme}`} style={style}>
        <div className="pl-intro pl-finish">
          <div className="pl-done-ico"><Icon name={outcome.completed ? 'check' : 'pause'} size={30} /></div>
          <h1 className="pl-name">{outcome.completed ? 'Well done' : 'Stopped early'}</h1>
          <p className="pl-sub">{fmtClock(outcome.duration)} of {r.name.toLowerCase()}{outcome.cycles ? ` · ${outcome.cycles} breaths` : ''}</p>
          {st.ask_calm && (
            <div className="pl-ask">
              <p>How do you feel now?</p>
              <CalmScale value={calmAfter} onChange={setCalmAfter} label="How you feel after" />
              {calmBefore && calmAfter && calmAfter > calmBefore && <p className="pl-shift">{calmAfter - calmBefore} {calmAfter - calmBefore === 1 ? 'step' : 'steps'} calmer than before.</p>}
            </div>
          )}
          <textarea className="pl-note" value={note} onChange={e => setNote(e.target.value)} placeholder="A note for your journal (optional): what came up, how it went…" rows={3} maxLength={10000} />
          {saveErr && <p className="pl-err"><Icon name="alert" size={15} />Not saved: {saveErr}</p>}
          <button type="button" className="pl-begin" onClick={save} disabled={saving}>{saving ? 'Saving…' : saveErr ? 'Try again' : 'Save'}</button>
          <button type="button" className="pl-link" onClick={() => onExit(back())}>Don't save</button>
        </div>
      </div>
    )
  }

  // ---------- result ----------
  if (stage === 'result' && result) {
    const next = result.program?.next
    const goalPct = result.goal_min > 0 ? Math.min(100, Math.round((result.day_minutes / result.goal_min) * 100)) : 0
    const doneTo = spec.program ? `/path/${spec.program}` : '/'
    return (
      <div className={`player ${theme}`} style={style}>
        <div className="pl-intro pl-result">
          <div className="pl-stats">
            <div className="pl-stat">
              <svg viewBox="0 0 44 44" className="pl-mini-ring"><circle cx="22" cy="22" r="18" /><circle cx="22" cy="22" r="18" className="fg" style={{ strokeDasharray: `${(goalPct / 100) * 113.1} 113.1` }} /></svg>
              <div><b>{Math.round(result.day_minutes)}</b>{result.goal_min > 0 && <span> / {result.goal_min}</span>} min today</div>
            </div>
            <div className="pl-stat"><Icon name="flame" size={26} className={result.streak > 0 ? 'flame-on' : ''} /><div><b>{result.streak}</b> {result.streak === 1 ? 'day' : 'days'} in a row</div></div>
          </div>
          {result.lesson_completed && (
            <div className="pl-banner">
              <Icon name="mountain" size={18} />
              <div><b>Lesson complete.</b>{result.program && <> {result.program.name}: {result.program.done} of {result.program.total}{result.program.done === result.program.total ? ' — all done.' : '.'}</>}</div>
            </div>
          )}
          {result.unlocked?.length > 0 && (
            <div className="pl-unlocked">
              {result.unlocked.map((a, i) => (
                <div key={a.key} className="pl-ach" style={{ animationDelay: `${0.15 + i * 0.12}s` }}>
                  <span className="pl-ach-ico"><Icon name={ACH_ICON[a.icon] || a.icon} size={22} /></span>
                  <div><div className="pl-ach-k">Milestone</div><b>{a.name}</b><div className="pl-ach-d">{a.description}</div></div>
                </div>
              ))}
            </div>
          )}
          {next && result.lesson_completed && (
            <Link className="pl-next" to={`/path/${next.program}/${next.key}`}>
              <span><span className="pl-ach-k">Next lesson</span><b>{next.title}</b><span className="pl-ach-d">{next.practice.minutes} min</span></span>
              <Icon name="chevron-right" size={20} />
            </Link>
          )}
          <div className="pl-sheet-btns">
            <button type="button" className="pl-btn" onClick={() => window.location.assign(window.location.pathname + window.location.search)}><Icon name="repeat" size={16} />Practise again</button>
            <button type="button" className="pl-btn primary" onClick={() => onExit(doneTo)}>Done</button>
          </div>
          <Link to={`/journal/${result.practice.id}`} className="pl-link">Open in the journal</Link>
        </div>
      </div>
    )
  }
  return null
}

function TimerRing({ p, total, warm }: { p: number; total: number; warm: number }) {
  const R = 46, C = 2 * Math.PI * R
  if (p < 0) {
    const f = warm > 0 ? (warm + p) / warm : 1
    return (
      <div className="pl-ring warm">
        <svg viewBox="0 0 100 100"><circle cx="50" cy="50" r={R} className="bg" /><circle cx="50" cy="50" r={R} className="fg" style={{ strokeDasharray: `${f * C} ${C}` }} /></svg>
        <div className="pl-ring-c"><div className="pl-ring-t">{Math.ceil(-p)}</div><div className="pl-ring-l">Settle in</div></div>
      </div>
    )
  }
  const f = total > 0 ? Math.min(1, p / total) : (p % 60) / 60
  return (
    <div className="pl-ring">
      <svg viewBox="0 0 100 100"><circle cx="50" cy="50" r={R} className="bg" /><circle cx="50" cy="50" r={R} className="fg" style={{ strokeDasharray: `${f * C} ${C}` }} /></svg>
      <div className="pl-ring-c"><div className="pl-ring-t">{fmtClock(total > 0 ? total - p : p)}</div><div className="pl-ring-l">{total > 0 ? 'remaining' : 'so far'}</div></div>
    </div>
  )
}
