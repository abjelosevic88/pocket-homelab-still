// The player's sounds, all synthesised with Web Audio: bells, breathing cues
// and background soundscapes. Nothing to download. Browsers (iPhone Safari
// above all) only start audio after a tap, so unlock() runs on "Begin".

let ctx: AudioContext | null = null
let master: GainNode | null = null
let volume = 0.8
let muted = false

export function audio(): AudioContext {
  if (!ctx) {
    const AC = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    ctx = new AC()
    master = ctx.createGain()
    master.gain.value = muted ? 0 : volume
    master.connect(ctx.destination)
  }
  return ctx
}

function out(): GainNode { audio(); return master! }

/** Call from a tap: resumes the context and plays a silent buffer (iOS needs one). */
export async function unlock() {
  const c = audio()
  try { await c.resume() } catch { /* ignore */ }
  const b = c.createBuffer(1, 1, 22050)
  const s = c.createBufferSource()
  s.buffer = b
  s.connect(c.destination)
  s.start(0)
  if ('speechSynthesis' in window) { try { const u = new SpeechSynthesisUtterance(''); u.volume = 0; speechSynthesis.speak(u) } catch { /* ignore */ } }
}

export function setVolume(v: number) {
  volume = Math.max(0, Math.min(1, v))
  if (master && ctx) master.gain.setTargetAtTime(muted ? 0 : volume, ctx.currentTime, 0.05)
  for (const a of musicEls) a.volume = muted ? 0 : volume
}

export function setMuted(m: boolean) {
  muted = m
  setVolume(volume)
}

export const isMuted = () => muted

// ---------- bells ----------

type Partial = [ratio: number, gain: number, decay: number]
const BELLS: Record<string, { f: number; partials: Partial[]; attack: number; gain: number }> = {
  bowl: { f: 196, attack: 0.02, gain: 0.5, partials: [[1, 1, 9], [2.76, 0.5, 6], [5.4, 0.25, 4], [8.93, 0.12, 3], [1.005, 0.6, 9]] },
  bell: { f: 523, attack: 0.003, gain: 0.35, partials: [[1, 1, 4], [2, 0.6, 3], [3, 0.3, 2.2], [4.2, 0.2, 1.6], [5.4, 0.12, 1.2]] },
  chime: { f: 880, attack: 0.002, gain: 0.25, partials: [[1, 1, 3], [2.4, 0.4, 2], [3.9, 0.2, 1.4], [1.5, 0.3, 2.5]] },
  gong: { f: 98, attack: 0.05, gain: 0.55, partials: [[1, 1, 10], [1.48, 0.7, 8], [2.1, 0.5, 7], [2.9, 0.35, 5], [4.1, 0.2, 4], [0.5, 0.4, 9]] },
  wood: { f: 740, attack: 0.001, gain: 0.5, partials: [[1, 1, 0.12], [2.3, 0.5, 0.08], [4.1, 0.2, 0.05]] },
}

/** Rings a bell (bowl, bell, chime, gong, wood); "none" or "" does nothing. */
export function bell(kind: string, strength = 1) {
  const b = BELLS[kind]
  if (!b) return
  const c = audio()
  const t = c.currentTime + 0.02
  const g = c.createGain()
  g.gain.value = b.gain * strength
  g.connect(out())
  for (const [ratio, gain, decay] of b.partials) {
    const o = c.createOscillator()
    o.type = 'sine'
    o.frequency.value = b.f * ratio
    const e = c.createGain()
    e.gain.setValueAtTime(0, t)
    e.gain.linearRampToValueAtTime(gain, t + b.attack)
    e.gain.exponentialRampToValueAtTime(0.0001, t + decay)
    o.connect(e).connect(g)
    o.start(t)
    o.stop(t + decay + 0.1)
  }
}

/** A short soft ping (the "bell" breathing cue). */
export function ping(high = true) {
  const c = audio()
  const t = c.currentTime + 0.01
  const o = c.createOscillator()
  o.type = 'sine'
  o.frequency.value = high ? 660 : 440
  const e = c.createGain()
  e.gain.setValueAtTime(0, t)
  e.gain.linearRampToValueAtTime(0.18, t + 0.01)
  e.gain.exponentialRampToValueAtTime(0.0001, t + 0.9)
  o.connect(e).connect(out())
  o.start(t)
  o.stop(t + 1)
}

/** A breathing phase as sound: a sine that swells up (in), down (out) or holds quietly. */
export function swell(type: string, seconds: number) {
  const c = audio()
  const t = c.currentTime + 0.01
  const d = Math.max(0.4, seconds)
  const o = c.createOscillator()
  o.type = 'sine'
  const f0 = 174.6
  const e = c.createGain()
  const lp = c.createBiquadFilter()
  lp.type = 'lowpass'
  lp.frequency.value = 900
  if (type === 'in' || type === 'in2') {
    o.frequency.setValueAtTime(f0, t)
    o.frequency.linearRampToValueAtTime(f0 * 1.5, t + d)
    e.gain.setValueAtTime(0.0001, t)
    e.gain.linearRampToValueAtTime(0.12, t + d * 0.8)
    e.gain.linearRampToValueAtTime(0.0001, t + d)
  } else if (type === 'out') {
    o.frequency.setValueAtTime(f0 * 1.5, t)
    o.frequency.linearRampToValueAtTime(f0, t + d)
    e.gain.setValueAtTime(0.0001, t)
    e.gain.linearRampToValueAtTime(0.12, t + d * 0.15)
    e.gain.linearRampToValueAtTime(0.0001, t + d)
  } else {
    o.frequency.value = type === 'hold' ? f0 * 1.5 : f0
    e.gain.setValueAtTime(0.0001, t)
    e.gain.linearRampToValueAtTime(0.035, t + 0.3)
    e.gain.linearRampToValueAtTime(0.0001, t + Math.min(d, 1.2))
  }
  o.connect(lp).connect(e).connect(out())
  o.start(t)
  o.stop(t + d + 0.1)
}

// ---------- ambient soundscapes ----------

function noiseBuffer(c: AudioContext, color: 'white' | 'pink' | 'brown', seconds = 4): AudioBuffer {
  const len = c.sampleRate * seconds
  const buf = c.createBuffer(2, len, c.sampleRate)
  for (let ch = 0; ch < 2; ch++) {
    const d = buf.getChannelData(ch)
    let last = 0
    let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0
    for (let i = 0; i < len; i++) {
      const w = Math.random() * 2 - 1
      if (color === 'white') d[i] = w * 0.5
      else if (color === 'brown') { last = (last + 0.02 * w) / 1.02; d[i] = last * 3.5 }
      else {
        b0 = 0.99886 * b0 + w * 0.0555179; b1 = 0.99332 * b1 + w * 0.0750759; b2 = 0.969 * b2 + w * 0.153852
        b3 = 0.8665 * b3 + w * 0.3104856; b4 = 0.55 * b4 + w * 0.5329522; b5 = -0.7616 * b5 - w * 0.016898
        d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11
        b6 = w * 0.115926
      }
    }
  }
  return buf
}

interface Ambient { stop: (fade?: number) => void }
let current: Ambient | null = null
const musicEls: HTMLAudioElement[] = []

/** Background names for pickers. */
export const AMBIENT_NAMES: Record<string, string> = { none: 'Silence', rain: 'Rain', ocean: 'Ocean waves', stream: 'Stream', brown: 'Brown noise', pink: 'Pink noise', fire: 'Fireplace' }
export const BELL_NAMES: Record<string, string> = { bowl: 'Singing bowl', bell: 'Bell', chime: 'Chime', gong: 'Gong', wood: 'Wood block', none: 'None' }

function loopNoise(c: AudioContext, color: 'white' | 'pink' | 'brown') {
  const s = c.createBufferSource()
  s.buffer = noiseBuffer(c, color, 6)
  s.loop = true
  return s
}

/** Starts a soundscape (fading in over a few seconds), replacing the one playing. */
export function startAmbient(kind: string) {
  stopAmbient(1.5)
  if (!kind || kind === 'none') return
  if (kind.startsWith('music:')) { current = startMusic(kind.slice(6)); return }
  const c = audio()
  const t = c.currentTime
  const bus = c.createGain()
  bus.gain.setValueAtTime(0.0001, t)
  bus.connect(out())
  const nodes: AudioScheduledSourceNode[] = []
  const timers: number[] = []
  const add = (src: AudioScheduledSourceNode) => { nodes.push(src); src.start() }
  let level = 0.5
  switch (kind) {
    case 'brown': { const s = loopNoise(c, 'brown'); s.connect(bus); add(s); level = 0.6; break }
    case 'pink': { const s = loopNoise(c, 'pink'); s.connect(bus); add(s); level = 0.35; break }
    case 'rain': {
      const s = loopNoise(c, 'pink')
      const hp = c.createBiquadFilter(); hp.type = 'highpass'; hp.frequency.value = 600
      const lp = c.createBiquadFilter(); lp.type = 'lowpass'; lp.frequency.value = 7000
      s.connect(hp).connect(lp).connect(bus); add(s)
      // drops: tiny filtered clicks now and then
      const drop = () => {
        const n = c.createBufferSource(); n.buffer = noiseBuffer(c, 'white', 0.03)
        const bp = c.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.value = 2000 + Math.random() * 4000; bp.Q.value = 8
        const g = c.createGain(); g.gain.value = 0.2 + Math.random() * 0.4
        n.connect(bp).connect(g).connect(bus); n.start()
        timers.push(window.setTimeout(drop, 30 + Math.random() * 180))
      }
      drop()
      level = 0.45
      break
    }
    case 'ocean': {
      const s = loopNoise(c, 'brown')
      const lp = c.createBiquadFilter(); lp.type = 'lowpass'; lp.frequency.value = 800
      const g = c.createGain(); g.gain.value = 0.3
      const lfo = c.createOscillator(); lfo.frequency.value = 0.09
      const depth = c.createGain(); depth.gain.value = 0.28
      lfo.connect(depth).connect(g.gain)
      const lfo2 = c.createOscillator(); lfo2.frequency.value = 0.09
      const fd = c.createGain(); fd.gain.value = 500
      lfo2.connect(fd).connect(lp.frequency)
      s.connect(lp).connect(g).connect(bus)
      add(s); add(lfo); add(lfo2)
      level = 0.8
      break
    }
    case 'stream': {
      const s = loopNoise(c, 'white')
      const bp = c.createBiquadFilter(); bp.type = 'bandpass'; bp.frequency.value = 1400; bp.Q.value = 0.7
      const lfo = c.createOscillator(); lfo.frequency.value = 0.35
      const fd = c.createGain(); fd.gain.value = 500
      lfo.connect(fd).connect(bp.frequency)
      s.connect(bp).connect(bus); add(s); add(lfo)
      level = 0.35
      break
    }
    case 'fire': {
      const s = loopNoise(c, 'brown')
      const lp = c.createBiquadFilter(); lp.type = 'lowpass'; lp.frequency.value = 500
      s.connect(lp).connect(bus); add(s)
      const crackle = () => {
        const n = c.createBufferSource(); n.buffer = noiseBuffer(c, 'white', 0.02)
        const hp = c.createBiquadFilter(); hp.type = 'highpass'; hp.frequency.value = 1500 + Math.random() * 3000
        const g = c.createGain(); g.gain.value = 0.15 + Math.random() * 0.6
        n.connect(hp).connect(g).connect(bus); n.start()
        timers.push(window.setTimeout(crackle, Math.random() < 0.2 ? 20 : 80 + Math.random() * 600))
      }
      crackle()
      level = 0.6
      break
    }
    default: bus.disconnect(); return
  }
  bus.gain.exponentialRampToValueAtTime(level, t + 4)
  current = {
    stop: (fade = 2) => {
      const n = c.currentTime
      timers.forEach(clearTimeout)
      bus.gain.cancelScheduledValues(n)
      bus.gain.setValueAtTime(Math.max(0.0001, bus.gain.value), n)
      bus.gain.exponentialRampToValueAtTime(0.0001, n + fade)
      window.setTimeout(() => { nodes.forEach(x => { try { x.stop() } catch { /* stopped */ } }); bus.disconnect() }, fade * 1000 + 100)
    },
  }
}

/** Plays a Navidrome playlist through Still, song after song, looping. */
function startMusic(playlist: string): Ambient {
  let stopped = false
  const el = new Audio()
  el.preload = 'auto'
  el.volume = muted ? 0 : volume
  musicEls.push(el)
  let songs: string[] = []
  let i = 0
  const next = () => {
    if (stopped || songs.length === 0) return
    el.src = `/api/v1/music/stream/${encodeURIComponent(songs[i % songs.length])}`
    i++
    el.play().catch(() => {})
  }
  el.addEventListener('ended', next)
  el.addEventListener('error', () => { if (songs.length > 1) window.setTimeout(next, 500) })
  fetch(`/api/v1/music/playlists/${encodeURIComponent(playlist)}`, { credentials: 'same-origin' }).then(r => r.json()).then(j => {
    songs = (j.songs || []).map((s: { id: string }) => s.id)
    next()
  }).catch(() => {})
  return {
    stop: (fade = 2) => {
      stopped = true
      const start = el.volume
      const steps = 20
      let k = 0
      const id = window.setInterval(() => {
        k++
        el.volume = Math.max(0, start * (1 - k / steps))
        if (k >= steps) { window.clearInterval(id); el.pause(); el.src = ''; musicEls.splice(musicEls.indexOf(el), 1) }
      }, (fade * 1000) / steps)
    },
  }
}

export function stopAmbient(fade = 2) {
  current?.stop(fade)
  current = null
}

/** Pauses or resumes everything (the context and any music). */
export function pauseAll(paused: boolean) {
  if (!ctx) return
  if (paused) { ctx.suspend().catch(() => {}); musicEls.forEach(a => a.pause()) }
  else { ctx.resume().catch(() => {}); musicEls.forEach(a => { if (a.src) a.play().catch(() => {}) }) }
}

// ---------- voice ----------

export function speak(text: string, voiceName = '') {
  if (muted || !('speechSynthesis' in window) || !text) return
  const u = new SpeechSynthesisUtterance(text)
  u.rate = 0.85
  u.pitch = 0.95
  u.volume = Math.min(1, volume + 0.1)
  const vs = speechSynthesis.getVoices()
  const v = (voiceName && vs.find(x => x.name === voiceName)) || vs.find(x => /en[-_]/i.test(x.lang) && /female|samantha|serena|karen|moira|daniel/i.test(x.name)) || vs.find(x => /^en/i.test(x.lang))
  if (v) u.voice = v
  speechSynthesis.cancel()
  speechSynthesis.speak(u)
}

export function stopSpeech() { if ('speechSynthesis' in window) speechSynthesis.cancel() }

export function vibrate(pattern: number | number[]) {
  try { navigator.vibrate?.(pattern) } catch { /* not supported */ }
}
