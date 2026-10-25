// Milestones: the level ladder from Ripple to Ocean and every achievement, reached or still ahead.
import { api, V1 } from '../lib/api'
import type { Achievements } from '../lib/types'
import { fmtDate } from '../lib/format'
import { GROUP_NAME, fmtMin } from '../lib/still'
import { Card, Loading, PageHeader, useAsync } from '../components/ui'
import { Icon } from '../components/icons'
import { useOnChange } from '../components/Layout'

export default function Milestones() {
  const { data: a, reload, error } = useAsync(() => api.get<Achievements>(`${V1}/achievements`), [])
  useOnChange(reload)
  if (error) return <div className="callout danger"><Icon name="alert" size={15} />{error}</div>
  if (!a) return <Loading />
  const groups = Object.keys(GROUP_NAME).map(g => [g, a.achievements.filter(x => x.group === g)] as const).filter(([, l]) => l.length)
  const pct = a.next_level ? Math.min(100, ((a.lifetime_min - a.level.minutes) / (a.next_level.minutes - a.level.minutes)) * 100) : 100
  return (
    <div className="stack loose">
      <PageHeader title="Milestones" sub={`${a.unlocked} of ${a.total} reached · ${fmtMin(a.lifetime_min)} of practice`} />
      <Card title="Levels" sub="Your lifetime minutes, from a ripple to the ocean.">
        <ol className="ladder">
          {a.levels.map(l => {
            const state = l.n < a.level.n ? 'past' : l.n === a.level.n ? 'current' : 'ahead'
            return (
              <li key={l.n} className={`ladder-step ${state}`}>
                <span className="ladder-n">{state === 'past' ? <Icon name="check" size={13} strokeWidth={2.4} /> : l.n}</span>
                <div className="grow col">
                  <b>{l.name}<span className="muted small"> · {l.minutes ? `${fmtMin(l.minutes)}` : 'your first breath'}</span></b>
                  <span className="muted small">{l.blurb}</span>
                  {state === 'current' && a.next_level && <><span className="progress"><span style={{ width: `${pct}%` }} /></span><span className="muted xsmall">{fmtMin(Math.max(0, a.next_level.minutes - a.lifetime_min))} to {a.next_level.name}</span></>}
                </div>
              </li>
            )
          })}
        </ol>
      </Card>
      {groups.map(([g, list]) => (
        <section key={g} className="stack">
          <h2 className="group-h">{GROUP_NAME[g]} <span className="muted">{list.filter(x => x.unlocked).length}/{list.length}</span></h2>
          <div className="ach-grid">
            {list.map(x => (
              <div key={x.key} className={`ach ${x.unlocked ? 'on' : 'off'}`} title={x.description}>
                <span className="ach-ico"><Icon name={x.unlocked ? x.icon : 'lock'} size={18} /></span>
                <div className="grow col">
                  <b>{x.name}</b>
                  <span className="muted small">{x.description}</span>
                  {x.unlocked_at && <span className="xsmall ach-date">{fmtDate(x.unlocked_at.slice(0, 10))}</span>}
                </div>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
