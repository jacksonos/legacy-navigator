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
export type Graph = { nodes: GraphNode[]; edges: GraphEdge[]; partial: boolean; reasons: Warning[] }

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
export const getGraph = (id: string) => call<Graph>(`/api/projects/${encodeURIComponent(id)}/graph`)
