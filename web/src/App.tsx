import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ApiError, getFile, getProject, loadSamples, search, uploadFiles,
  type Project, type Ref, type SearchResult, type Warning,
} from './api'
import CodeViewer from './CodeViewer'
import { I18nContext, detectLang, useT, type Lang } from './i18n'

const KINDS = ['', 'data', 'paragraph', 'section', 'file', 'program']
const REF_KINDS = ['write', 'read', 'io'] as const
type Open = (file: string, line?: number) => void

function Warnings({ list }: { list: Warning[] }) {
  const t = useT()
  if (!list.length) return null
  return (
    <ul className="warnings">
      {list.map((w, i) => (
        <li key={i}>
          <strong>{t('warn.' + w.kind)}</strong>
          {w.detail && <> — {w.detail}</>}
          {w.file && <> ({w.file}{w.line ? `:${w.line}` : ''})</>}
        </li>
      ))}
    </ul>
  )
}

function FilesPanel({ project, busy, onUpload, onSamples, open }: {
  project: Project | null
  busy: boolean
  onUpload: (f: File[]) => void
  onSamples: () => void
  open: Open
}) {
  const t = useT()
  const [over, setOver] = useState(false)
  return (
    <section className="panel" aria-labelledby="h-files">
      <h2 id="h-files">{t('files')}</h2>
      <div
        className={over ? 'drop over' : 'drop'}
        onDragOver={(e) => { e.preventDefault(); setOver(true) }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => { e.preventDefault(); setOver(false); if (e.dataTransfer.files.length) onUpload([...e.dataTransfer.files]) }}
      >
        <p>{t('dropHint')}</p>
        <label className="btn">
          {t('chooseFiles')}
          <input
            type="file" multiple accept=".cbl,.cob,.cpy,.txt"
            onChange={(e) => { if (e.target.files?.length) onUpload([...e.target.files]); e.target.value = '' }}
          />
        </label>
        <button type="button" className="btn" onClick={onSamples} disabled={busy}>{t('useSamples')}</button>
        <p className="muted">{t('accepted')}</p>
      </div>
      {project ? (
        <>
          {project.warnings.length > 0 && (
            <div>
              <h3>{t('projectWarnings')}</h3>
              <Warnings list={project.warnings} />
            </div>
          )}
          <ul className="files">
            {project.files.map((f) => (
              <li key={f.name}>
                <button type="button" className="link name" onClick={() => open(f.name)}>{f.name}</button>
                <div className="muted">
                  {t('programId')}: {f.programId || '—'} · {f.lines} {t('lines')} · {f.paragraphs} {t('paragraphs')} · {f.dataItems} {t('dataItems')}
                </div>
                <Warnings list={f.warnings} />
              </li>
            ))}
          </ul>
        </>
      ) : (
        <p className="muted">{t('noProject')}</p>
      )}
    </section>
  )
}

function SearchPanel({ projectId, open, fail }: { projectId: string | null; open: Open; fail: (e: unknown) => void }) {
  const t = useT()
  const [q, setQ] = useState('')
  const [kind, setKind] = useState('')
  const [raw, setRes] = useState<SearchResult | null>(null)
  const active = !!projectId && !!q.trim()
  const res = active ? raw : null

  useEffect(() => {
    if (!projectId || !q.trim()) return
    const ctl = new AbortController()
    const h = setTimeout(() => {
      search(projectId, q.trim(), kind, ctl.signal).then(setRes).catch((e) => { if (!ctl.signal.aborted) fail(e) })
    }, 250)
    return () => { clearTimeout(h); ctl.abort() }
  }, [projectId, q, kind, fail])

  const link = (file: string, line: number, extra = '') => (
    <button type="button" className="link" onClick={() => open(file, line)}>
      {file}:{line}{extra && ` (${extra})`}
    </button>
  )
  const results = res?.results ?? []
  const lines = res?.lines ?? []

  return (
    <section className="panel" aria-labelledby="h-search">
      <h2 id="h-search">{t('search')}</h2>
      <div className="row">
        <label>
          {t('searchLabel')}
          <input type="search" value={q} placeholder={t('searchPlaceholder')} disabled={!projectId} onChange={(e) => setQ(e.target.value)} />
        </label>
        <label>
          {t('kind')}
          <select value={kind} disabled={!projectId} onChange={(e) => setKind(e.target.value)}>
            {KINDS.map((k) => <option key={k} value={k}>{t('kind.' + (k || 'all'))}</option>)}
          </select>
        </label>
      </div>
      <div aria-live="polite">
        {res && !results.length && !lines.length && <p className="muted">{t('noResults')}</p>}
        {results.length > 0 && (
          <>
            <h3>{t('symbols')}</h3>
            <ul className="hits">
              {results.map((s, i) => (
                <li key={i}>
                  <div><strong>{s.name}</strong> <span className="tag">{t('kind.' + s.kind)}</span></div>
                  <div>{t('definedAt')} {link(s.def.file, s.def.line)}</div>
                  {REF_KINDS.map((rk) => {
                    const rs: Ref[] = (s.refs ?? []).filter((r) => r.kind === rk)
                    return rs.length > 0 && (
                      <div key={rk} className="refs">
                        <span className={'tag ' + rk}>{t('ref.' + rk)}</span>
                        {rs.map((r, j) => <span key={j}>{link(r.file, r.line, r.paragraph)}</span>)}
                      </div>
                    )
                  })}
                </li>
              ))}
            </ul>
          </>
        )}
        {lines.length > 0 && (
          <>
            <h3>{t('textHits')}</h3>
            <ul className="hits">
              {lines.map((l, i) => (
                <li key={i}>
                  {link(l.file, l.line)} <code className="snippet">{l.text.trim()}</code>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </section>
  )
}

function Shell({ lang, setLang }: { lang: Lang; setLang: (l: Lang) => void }) {
  const t = useT()
  const [project, setProject] = useState<Project | null>(null)
  const [busy, setBusy] = useState(() => !!new URLSearchParams(location.search).get('p'))
  const [error, setError] = useState<{ status: number; msg: string } | null>(null)
  const [gone, setGone] = useState(false)
  const [view, setView] = useState<{ file: string; lines: string[]; highlight?: number } | null>(null)
  const viewReq = useRef(0)

  const fail = useCallback((e: unknown) => {
    setError(e instanceof ApiError ? { status: e.status, msg: e.message } : { status: -1, msg: '' })
  }, [])
  const known = error && [0, 400, 404, 413].includes(error.status)
  const errText = error && (t(known ? 'err.' + error.status : 'err.other') + (error.msg ? ` (${error.msg})` : ''))

  const setPid = (id: string | null) => {
    const u = new URL(location.href)
    if (id) u.searchParams.set('p', id)
    else u.searchParams.delete('p')
    history.replaceState(null, '', u)
  }

  const run = async (p: Promise<Project>) => {
    setBusy(true); setError(null); setGone(false)
    try {
      const pr = await p
      setProject(pr); setView(null); setPid(pr.id)
    } catch (e) {
      fail(e)
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    const id = new URLSearchParams(location.search).get('p')
    if (!id) return
    getProject(id)
      .then(setProject)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) { setPid(null); setGone(true) } else fail(e)
      })
      .finally(() => setBusy(false))
  }, [fail])

  const open: Open = useCallback((file, line) => {
    if (!project) return
    const req = ++viewReq.current
    getFile(project.id, file)
      .then((f) => { if (req === viewReq.current) setView({ file, lines: f.lines, highlight: line }) })
      .catch(fail)
  }, [project, fail])

  return (
    <>
      <header>
        <h1>{t('title')}</h1>
        <label className="lang">
          {t('language')}
          <select value={lang} onChange={(e) => setLang(e.target.value as Lang)}>
            <option value="en">EN</option>
            <option value="es">ES</option>
          </select>
        </label>
      </header>
      <div className="status" aria-live="polite">
        {busy && <p>{t('loading')}</p>}
        {gone && <p className="err">{t('projectGone')}</p>}
        {errText && <p className="err">{errText}</p>}
      </div>
      <main>
        <FilesPanel
          project={project} busy={busy} open={open}
          onUpload={(f) => run(uploadFiles(f))} onSamples={() => run(loadSamples())}
        />
        <section className="panel viewer-panel" aria-labelledby="h-viewer">
          <h2 id="h-viewer">{t('viewer')}</h2>
          {view ? <CodeViewer {...view} /> : <p className="muted">{t('noFile')}</p>}
        </section>
        <SearchPanel projectId={project?.id ?? null} open={open} fail={fail} />
      </main>
    </>
  )
}

export default function App() {
  const [lang, setLang] = useState<Lang>(detectLang)
  useEffect(() => {
    document.documentElement.lang = lang
    try { localStorage.setItem('lang', lang) } catch { /* storage unavailable */ }
  }, [lang])
  return (
    <I18nContext.Provider value={lang}>
      <Shell lang={lang} setLang={setLang} />
    </I18nContext.Provider>
  )
}
