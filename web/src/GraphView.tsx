import { useEffect, useRef, useState } from 'react'
import type { Graph, GraphNode, GraphQuery } from './api'
import { useT } from './i18n'

const esc = (s: string) => s.replace(/[#"<>&]/g, (c) => `#${c.charCodeAt(0)};`)

/** Builds the Mermaid source; `byMermaidId` maps generated ids (n0, n1…) back to nodes. */
function toMermaid(g: Graph) {
  const ids = new Map<string, string>()
  g.nodes.forEach((n, i) => ids.set(n.id, `n${i}`))
  const hot = new Set<string>(g.node && ids.has(g.node) ? [g.node] : []) // focus center
  const out = ['flowchart LR']
  const def = (n: GraphNode) => `${ids.get(n.id)}["${esc(n.label)}"]:::${n.kind}`
  const progOf = (n: GraphNode) =>
    n.kind === 'program' ? n.id : n.kind === 'section' || n.kind === 'paragraph' ? n.id.split(':')[0] : ''
  // one subgraph per program, derived from the node ids (the program node itself may be absent); system view: none
  const groups = new Map<string, GraphNode[]>()
  g.nodes.forEach((n) => {
    const p = g.view === 'system' ? '' : progOf(n)
    if (p) groups.set(p, [...(groups.get(p) ?? []), n])
  })
  let sg = 0
  groups.forEach((members, p) => {
    out.push(`subgraph sg${sg++}["${esc(p)}"]`)
    members.forEach((n) => out.push('  ' + def(n)))
    out.push('end')
  })
  g.nodes.filter((n) => g.view === 'system' || !progOf(n)).forEach((n) => out.push(def(n)))
  g.edges.forEach((e) => {
    const a = ids.get(e.from), b = ids.get(e.to)
    if (e.kind === 'call' && !e.uncertain) out.push(`${a} ==> ${b}`)
    else {
      const label = e.kind === 'goto' ? (e.uncertain ? 'goto ?' : 'goto') : e.uncertain ? (e.kind === 'call' ? 'call ?' : '?') : ''
      out.push(label ? `${a} -. "${label}" .-> ${b}` : `${a} --> ${b}`)
    }
  })
  out.push(
    'classDef program fill:#1d4ed8,stroke:#1e3a8a,color:#fff',
    'classDef section fill:#7c3aed,stroke:#4c1d95,color:#fff',
    'classDef external fill:#9a3412,stroke:#7c2d12,color:#fff',
    'classDef dynamic fill:#6b7280,stroke:#374151,color:#fff',
    'classDef unresolved stroke-dasharray:6 4,stroke-width:2px',
    'classDef highlight stroke:#f59e0b,stroke-width:4px',
  )
  const unres = g.nodes.filter((n) => !n.resolved).map((n) => ids.get(n.id))
  if (unres.length) out.push(`class ${unres.join(',')} unresolved`)
  if (hot.size) out.push(`class ${[...hot].map((h) => ids.get(h)).join(',')} highlight`)
  // dimming is applied to the rendered node groups (shape + label) in GraphView
  return { code: out.join('\n'), hot, byMermaidId: new Map(g.nodes.map((n) => [ids.get(n.id)!, n])) }
}

let seq = 0
let ready: Promise<typeof import('mermaid').default> | null = null
function loadMermaid() {
  ready ??= import('mermaid').then(({ default: m }) => {
    m.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'default',
      maxEdges: 500, // headroom above the server cap (300)
      flowchart: { useMaxWidth: false },
    })
    return m
  })
  return ready
}

export default function GraphView({ graph, onNode, onQuery }: { graph: Graph; onNode: (n: GraphNode) => void; onQuery: (q: GraphQuery) => void }) {
  const t = useT()
  const box = useRef<HTMLDivElement>(null)
  const [fit, setFit] = useState(true)
  const fitRef = useRef(fit)
  const [state, setState] = useState<'loading' | 'ok' | 'error'>('loading')
  const [picked, setPicked] = useState<{ g: Graph; n: GraphNode } | null>(null)
  const sel = picked?.g === graph ? picked.n : null // selection belongs to the graph it was made on
  const act = useRef({ onQuery, graph })
  useEffect(() => { act.current = { onQuery, graph }; fitRef.current = fit })
  const center = graph.node || sel?.id
  const progs = graph.programs ?? []

  useEffect(() => {
    if (!graph.nodes.length) return
    let live = true
    const { code, hot, byMermaidId } = toMermaid(graph)
    loadMermaid()
      .then((m) => m.render(`mmd${++seq}`, code))
      .then(({ svg }) => {
        if (!live || !box.current) return
        box.current.innerHTML = svg
        let first: SVGGElement | null = null
        box.current.querySelectorAll<SVGGElement>('g.node').forEach((el) => {
          const n = byMermaidId.get(/flowchart-(n\d+)-\d+$/.exec(el.id)?.[1] ?? '')
          if (!n) return
          if (hot.has(n.id)) first = el
          const pick = () => {
            if (act.current.graph.view === 'system' && n.kind === 'program') return act.current.onQuery({ view: 'program', program: n.id })
            box.current?.querySelectorAll('g.node.sel').forEach((x) => x.classList.remove('sel'))
            el.classList.add('sel')
            setPicked({ g: act.current.graph, n })
          }
          el.style.cursor = 'pointer'
          el.setAttribute('tabindex', '0')
          el.setAttribute('role', 'button')
          el.addEventListener('click', pick)
          el.addEventListener('keydown', (ev) => { if (ev.key === 'Enter' || ev.key === ' ') { ev.preventDefault(); pick() } })
        })
        if (first && !fitRef.current) (first as SVGGElement).scrollIntoView({ block: 'center', inline: 'center' })
        setState('ok')
      })
      .catch(() => { if (live) setState('error') })
    return () => { live = false }
  }, [graph])

  if (!graph.nodes.length) return <p className="muted">{t('mapEmpty')}</p>
  return (
    <>
      {state === 'loading' && <p className="muted">{t('mapLoading')}</p>}
      {state === 'error' && <p className="err">{t('mapError')}</p>}
      <div className="map-bar" role="group" aria-label={t('mapView')}>
        <button type="button" className="btn" aria-pressed={graph.view === 'system'} onClick={() => onQuery({ view: 'system' })}>{t('viewSystem')}</button>
        <button type="button" className="btn" aria-pressed={graph.view === 'program'} disabled={!progs.length}
          onClick={() => onQuery({ view: 'program', program: graph.program || progs[0] })}>{t('viewProgram')}</button>
        <button type="button" className="btn" aria-pressed={graph.view === 'focus'} disabled={!center}
          onClick={() => onQuery({ view: 'focus', node: center, depth: graph.depth || 1 })}>{t('viewFocus')}</button>
        <label>
          {t('mapProgram')}
          <select value={graph.program} onChange={(e) => onQuery({ view: 'program', program: e.target.value })}>
            {!graph.program && <option value="" />}
            {progs.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </label>
        {graph.view === 'focus' && (
          <label>
            {t('mapDepth')}
            <select value={graph.depth} onChange={(e) => onQuery({ view: 'focus', node: graph.node, depth: Number(e.target.value) })}>
              {[1, 2, 3].map((d) => <option key={d} value={d}>{d}</option>)}
            </select>
          </label>
        )}
      </div>
      <button type="button" className="btn fit-btn" aria-pressed={!fit} onClick={() => setFit(!fit)}>
        {t(fit ? 'mapFit' : 'map100')}
      </button>
      <div className="map-wrap">
        <div className={fit ? 'map fit' : 'map'} ref={box} />
      </div>
      <p className="map-bar" aria-live="polite">
        {sel ? (
          <>
            <span>{t('selected')}: <strong>{sel.label}</strong></span>
            {sel.file && <button type="button" className="btn" onClick={() => onNode(sel)}>{t('openCode')}</button>}
            <button type="button" className="btn" onClick={() => onQuery({ view: 'focus', node: sel.id, depth: graph.depth || 1 })}>{t('focusHere')}</button>
          </>
        ) : <span className="muted">{t('selectHint')}</span>}
      </p>
    </>
  )
}

export function Legend() {
  const t = useT()
  return (
    <p className="legend muted">
      {t('legend')}: <span>── {t('legend.perform')}</span> <span>┄ {t('legend.goto')}</span> <span>━ {t('legend.call')}</span>{' '}
      <span>┄ ? {t('legend.uncertain')}</span> <span className="unres">▭ {t('legend.unresolved')}</span>
    </p>
  )
}
