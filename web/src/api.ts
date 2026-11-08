export type Warning = { kind: string; detail: string; file: string; line: number }
export type FileSummary = { name: string; lines: number; programId: string; paragraphs: number; dataItems: number; warnings: Warning[] }
export type Project = { id: string; files: FileSummary[]; warnings: Warning[] }
export type Ref = { kind: 'read' | 'write' | 'io'; file: string; line: number; paragraph: string }
export type SymbolHit = { name: string; kind: string; def: { file: string; line: number }; refs: Ref[] }
export type LineHit = { file: string; line: number; text: string }
export type SearchResult = { results: SymbolHit[]; lines: LineHit[] }

export type GraphNode = {
  id: string; label: string; kind: 'program' | 'section' | 'paragraph' | 'external' | 'dynamic'
  file: string; line: number; resolved: boolean
}
export type GraphEdge = { from: string; to: string; kind: 'perform' | 'goto' | 'call'; file: string; line: number; uncertain: boolean }
export type GraphQuery = { view?: 'system' | 'program' | 'focus'; program?: string; node?: string; depth?: number }
export type Graph = {
  view: 'system' | 'program' | 'focus'; program: string; node: string; depth: number; programs: string[]
  nodes: GraphNode[]; edges: GraphEdge[]; partial: boolean; reasons: Warning[] }

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function call<T>(url: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, init)
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError(0, '')
  }
  if (!res.ok) {
    let msg = ''
    try {
      msg = ((await res.json()) as { error?: string }).error ?? ''
    } catch {
      /* body is not JSON */
    }
    throw new ApiError(res.status, msg)
  }
  return res.json() as Promise<T>
}

export const uploadFiles = (files: File[]) => {
  const fd = new FormData()
  files.forEach((f) => fd.append('files', f))
  return call<Project>('/api/projects', { method: 'POST', body: fd })
}
export const loadSamples = () => call<Project>('/api/projects?samples=1', { method: 'POST' })
export const getProject = (id: string) => call<Project>(`/api/projects/${encodeURIComponent(id)}`)
export const getFile = (id: string, name: string) =>
  call<{ name: string; lines: string[] }>(`/api/projects/${encodeURIComponent(id)}/files/${encodeURIComponent(name)}`)
export const search = (id: string, q: string, kind: string, signal?: AbortSignal) =>
  call<SearchResult>(
    `/api/projects/${encodeURIComponent(id)}/search?q=${encodeURIComponent(q)}${kind ? `&kind=${kind}` : ''}`,
    { signal },
  )
export const getGraph = (id: string, q: GraphQuery = {}) => {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) if (v !== undefined && v !== '') p.set(k, String(v))
  return call<Graph>(`/api/projects/${encodeURIComponent(id)}/graph${p.size ? '?' + p : ''}`)
}

export type Provider = { id: string; model: string }
export type Evidence = { file: string; startLine: number; endLine: number; paragraph: string; text: string }
export type AskResponse = {
  mode: 'llm' | 'evidence'; provider: string; answer: string
  citations: { file: string; line: number; endLine: number }[]; unverified: string[]; evidence: Evidence[]
}
export const getProviders = () => call<{ providers: Provider[] }>('/api/providers')
export const ask = (id: string, question: string, lang: string, provider?: string) =>
  call<AskResponse>(`/api/projects/${encodeURIComponent(id)}/ask`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ question, lang, ...(provider ? { provider } : {}) }),
  })
