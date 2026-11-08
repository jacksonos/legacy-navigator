import { useEffect, useRef, useState } from 'react'
import type { Graph, GraphNode } from './api'
import { useT } from './i18n'

const esc = (s: string) => s.replace(/[#"<>&]/g, (c) => `#${c.charCodeAt(0)};`)

/** Builds the Mermaid source; `ids` maps generated ids (n0, n1…) back to nodes. */
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
  g.nodes.forEach((n) => out.push(`${ids.get(n.id)}["${esc(n.label)}"]:::${n.kind}`))
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
    'classDef dim opacity:.3',
  )
  const unres = g.nodes.filter((n) => !n.resolved).map((n) => ids.get(n.id))
  if (unres.length) out.push(`class ${unres.join(',')} unresolved`)
  if (hot.size) {
    out.push(`class ${[...hot].map((h) => ids.get(h)).join(',')} highlight`)
    const cold = g.nodes.filter((n) => !hot.has(n.id)).map((n) => ids.get(n.id))
    if (cold.length) out.push(`class ${cold.join(',')} dim`)
  }
  return { code: out.join('\n'), byMermaidId: new Map(g.nodes.map((n) => [ids.get(n.id)!, n])) }
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
  const [state, setState] = useState<'loading' | 'ok' | 'error'>('loading')
  const onNodeRef = useRef(onNode)
  useEffect(() => { onNodeRef.current = onNode })

  useEffect(() => {
    if (!graph.nodes.length) return
    let live = true
    const { code, byMermaidId } = toMermaid(graph, focus)
    loadMermaid()
      .then((m) => m.render(`mmd${++seq}`, code))
      .then(({ svg }) => {
        if (!live || !box.current) return
        box.current.innerHTML = svg
        box.current.querySelectorAll<SVGGElement>('g.node').forEach((el) => {
          const n = byMermaidId.get(/^flowchart-(n\d+)-/.exec(el.id)?.[1] ?? '')
          if (!n) return
          const go = () => onNodeRef.current(n)
          el.style.cursor = 'pointer'
          el.setAttribute('tabindex', '0')
          el.setAttribute('role', 'button')
          el.addEventListener('click', go)
          el.addEventListener('keydown', (ev) => { if (ev.key === 'Enter' || ev.key === ' ') { ev.preventDefault(); go() } })
        })
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
      <div className="map" ref={box} />
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
