import { Fragment, useState, type ReactNode } from 'react'
import { ask, type AskResponse, type Provider } from './api'
import { useCount, useT } from './i18n'
import type { Lang } from './i18n'

type Open = (file: string, line?: number) => void
const CITE = /^(.+):(\d+)(?:-\d+)?$/
const NAMES: Record<string, string> = { anthropic: 'Claude', openai: 'OpenAI', gemini: 'Gemini', openrouter: 'OpenRouter' }
const label = (p: Provider) => `${NAMES[p.id] ?? p.id} (${p.model})`

/** Plain text with every [file:line], [file:a-b] or comma list inside one bracket turned into viewer links. */
function Answer({ text, open }: { text: string; open: Open }) {
  const out: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(/\[([^[\]]+)\]/g)) {
    const parts = m[1].split(',').map((s) => s.trim())
    if (!parts.every((p) => CITE.test(p))) continue
    out.push(text.slice(last, m.index), '[')
    parts.forEach((p, i) => {
      const [, file, line] = p.match(CITE)!
      out.push(
        <Fragment key={`${m.index}-${i}`}>
          {i > 0 && ', '}
          <button type="button" className="link" onClick={() => open(file, +line)}>{p}</button>
        </Fragment>,
      )
    })
    out.push(']')
    last = m.index + m[0].length
  }
  out.push(text.slice(last))
  return <p className="answer">{out}</p>
}

export default function AskPanel({ projectId, lang, providers, open, fail, showMap }: {
  projectId: string | null; lang: Lang; providers: Provider[]; open: Open; fail: (e: unknown) => void; showMap: (l: string, file: string) => void
}) {
  const t = useT()
  const count = useCount()
  const [q, setQ] = useState('')
  const [prov, setProv] = useState('')
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<AskResponse | null>(null)
  const can = !!projectId && !busy && !!q.trim()

  const submit = async () => {
    if (!can) return
    setBusy(true); setRes(null)
    try {
      setRes(await ask(projectId, q.trim(), lang, providers.length ? prov || providers[0].id : undefined))
    } catch (e) {
      fail(e)
    } finally {
      setBusy(false)
    }
  }
  const llm = res?.mode === 'llm'
  const used = providers.find((p) => p.id === res?.provider)

  return (
    <section className="panel ask" aria-labelledby="h-ask">
      <h2 id="h-ask">{t('ask')}</h2>
      <form onSubmit={(e) => { e.preventDefault(); submit() }}>
        {providers.length > 0 ? (
          <label>
            {t('provider')}
            <select value={prov || providers[0].id} onChange={(e) => setProv(e.target.value)}>
              {providers.map((p) => <option key={p.id} value={p.id}>{label(p)}</option>)}
            </select>
          </label>
        ) : <p className="muted">{t('noProvider')}</p>}
        <label>
          {t('askLabel')}
          <textarea
            rows={3} maxLength={500} value={q} placeholder={t('askPlaceholder')} disabled={!projectId}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); submit() } }}
          />
        </label>
        <div><button type="submit" className="btn" disabled={!can}>{t('askSubmit')}</button> <span className="muted">{t('askHint')}</span></div>
        {providers.length > 0 && <p className="muted">{t('askPrivacy')}</p>}
      </form>
      <div aria-live="polite">
        {busy && <p>{t('asking')}</p>}
        {res && (
          <>
            {llm ? (
              <>
                <h3>{t('answer')}</h3>
                <Answer text={res.answer} open={open} />
                <p className="muted">{t('answeredBy')} {used ? label(used) : res.provider}</p>
              </>
            ) : <h3>{res.evidence.length ? t('evidenceOnly') : t('notFound')}</h3>}
            {res.unverified?.length > 0 && (
              <div className="warn-box" role="alert">
                {count('unverifiedWarn', res.unverified.length)}: <code>{res.unverified.join(', ')}</code>
              </div>
            )}
            {res.evidence.length > 0 && (
              <details className="evidence" open={!llm}>
                <summary>{t('evidenceTitle')} ({res.evidence.length})</summary>
                <ul className="hits">
                  {res.evidence.map((ev, i) => (
                    <li key={i}>
                      <button type="button" className="link" onClick={() => open(ev.file, ev.startLine)}>
                        {ev.file}:{ev.startLine}-{ev.endLine} ({ev.paragraph})
                      </button>
                      {ev.paragraph && <button type="button" className="link map-btn" onClick={() => showMap(ev.paragraph, ev.file)}>{t('showInMap')}</button>}
                      <pre>{ev.text}</pre>
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </>
        )}
      </div>
    </section>
  )
}
