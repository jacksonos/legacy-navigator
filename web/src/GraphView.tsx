import { useEffect, useRef, useState } from 'react'
import type { Graph, GraphNode } from './api'
import { useT } from './i18n'

const esc = (s: string) => s.replace(/[#"<>&]/g, (c) => `#${c.charCodeAt(0)};`)

/** Builds the Mermaid source; `byMermaidId` maps generated ids (n0, n1…) back to nodes. */
function toMermaid(g: Graph, focus: string | null) {
  const ids = new Map<string, string>()
  g.nodes.forEach((n, i) => ids.set(n.id, `n${i}`))
  const hot = new Set<string>()
  if (focus) {
    const seeds = g.nodes.filter((n) => n.label === focus && (n.kind === 'paragraph' || n.kind === 'section')).map((n) => n.id)
    seeds.forEach((s) => hot.add(s))
    g.edges.forEach((e) => {
      if (seeds.includes(e.from)) hot.add(e.to)
      if (seeds.includes(e.to)) hot.add(e.from)
    })
  }
  const out = ['flowchart LR']
  const def = (n: GraphNode) => `${ids.get(n.id)}["${esc(n.label)}"]:::${n.kind}`
  const progOf = (n: GraphNode) =>
    n.kind === 'program' ? n.id : n.kind === 'section' || n.kind === 'paragraph' ? n.id.split(':')[0] : ''
  g.nodes.filter((n) => n.kind === 'program').forEach((p, i) => {
    out.push(`subgraph sg${i}["${esc(p.label)}"]`)
    g.nodes.filter((n) => progOf(n) === p.id).forEach((n) => out.push('  ' + def(n)))
    out.push('end')
  })
  g.nodes.filter((n) => !progOf(n)).forEach((n) => out.push(def(n)))
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
      flowchart: { useMaxWidth: false },
    })
    return m
  })
  return ready
}

export default function GraphView({ graph, focus, onNode }: { graph: Graph; focus: string | null; onNode: (n: GraphNode) => void }) {
  const t = useT()
  const box = useRef<HTMLDivElement>(null)
  const [fit, setFit] = useState(true)
  const fitRef = useRef(fit)
  const [state, setState] = useState<'loading' | 'ok' | 'error'>('loading')
  const onNodeRef = useRef(onNode)
  useEffect(() => { onNodeRef.current = onNode; fitRef.current = fit })

  useEffect(() => {
    if (!graph.nodes.length) return
    let live = true
    const { code, hot, byMermaidId } = toMermaid(graph, focus)
    loadMermaid()
      .then((m) => m.render(`mmd${++seq}`, code))
      .then(({ svg }) => {
        if (!live || !box.current) return
        box.current.innerHTML = svg
        let first: SVGGElement | null = null
        box.current.querySelectorAll<SVGGElement>('g.node').forEach((el) => {
          const n = byMermaidId.get(/flowchart-(n\d+)-\d+$/.exec(el.id)?.[1] ?? '')
          if (!n) return
          if (hot.size) {
            if (n.label === focus && n.kind !== 'program') first = el // the searched paragraph itself, not just a neighbour
            else if (hot.has(n.id)) first ??= el
            else el.classList.add('dim')
          }
          const go = () => onNodeRef.current(n)
          el.style.cursor = 'pointer'
          el.setAttribute('tabindex', '0')
          el.setAttribute('role', 'button')
          el.addEventListener('click', go)
          el.addEventListener('keydown', (ev) => { if (ev.key === 'Enter' || ev.key === ' ') { ev.preventDefault(); go() } })
        })
        if (first && !fitRef.current) (first as SVGGElement).scrollIntoView({ block: 'center', inline: 'center' })
        setState('ok')
      })
      .catch(() => { if (live) setState('error') })
    return () => { live = false }
  }, [graph, focus])

  if (!graph.nodes.length) return <p className="muted">{t('mapEmpty')}</p>
  return (
    <>
      {state === 'loading' && <p className="muted">{t('mapLoading')}</p>}
      {state === 'error' && <p className="err">{t('mapError')}</p>}
      <button type="button" className="btn fit-btn" aria-pressed={!fit} onClick={() => setFit(!fit)}>
        {t(fit ? 'mapFit' : 'map100')}
      </button>
      <div className="map-wrap">
        <div className={fit ? 'map fit' : 'map'} ref={box} />
      </div>
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
