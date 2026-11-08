import { createContext, useContext } from 'react'

const en = {
  title: 'Legacy Code Navigator',
  language: 'Language',
  files: 'Files',
  dropHint: 'Drop COBOL files here or choose them',
  accepted: 'Accepted: .cbl, .cob, .cpy, .txt (max 1 MB per file, 10 MB per project)',
  chooseFiles: 'Choose files',
  useSamples: 'Use samples',
  loading: 'Loading…',
  noProject: 'No project yet. Upload files or use the samples.',
  projectWarnings: 'Project warnings',
  programId: 'PROGRAM-ID',
  lines: 'lines',
  paragraphs: 'paragraphs',
  dataItems: 'data items',
  viewer: 'Code viewer',
  noFile: 'Select a file or a search result to view its code.',
  search: 'Search',
  searchLabel: 'Symbol or text',
  searchPlaceholder: 'e.g. SALDO',
  kind: 'Type',
  'kind.all': 'All',
  'kind.data': 'Data',
  'kind.paragraph': 'Paragraph',
  'kind.section': 'Section',
  'kind.file': 'File',
  'kind.program': 'Program',
  symbols: 'Symbols',
  textHits: 'Text matches',
  noResults: 'No results.',
  definedAt: 'Defined at',
  'ref.write': 'Write',
  'ref.read': 'Read',
  'ref.io': 'I/O',
  'warn.missing_copy': 'Missing COPY',
  'warn.dynamic_call': 'Dynamic CALL',
  'warn.unresolved_call': 'Unresolved CALL',
  'warn.alter': 'ALTER statement',
  'warn.goto_depending': 'GO TO DEPENDING',
  'warn.not_cobol': 'Does not look like COBOL',
  'warn.free_format': 'Free-format source',
  'warn.unsupported': 'Unsupported construct',
  'err.0': 'Cannot reach the server.',
  'err.400': 'Invalid request.',
  'err.404': 'Not found.',
  'err.413': 'Upload too large (limit: 1 MB per file, 10 MB per project).',
  'err.other': 'Unexpected error.',
  projectGone: 'The project in the URL no longer exists.',
} as const

export type Key = keyof typeof en
export type Lang = 'en' | 'es'

const es: Record<Key, string> = {
  title: 'Navegador de código legado',
  language: 'Idioma',
  files: 'Archivos',
  dropHint: 'Suelta aquí archivos COBOL o elígelos',
  accepted: 'Admitidos: .cbl, .cob, .cpy, .txt (máx. 1 MB por archivo, 10 MB por proyecto)',
  chooseFiles: 'Elegir archivos',
  useSamples: 'Usar ejemplos',
  loading: 'Cargando…',
  noProject: 'Aún no hay proyecto. Sube archivos o usa los ejemplos.',
  projectWarnings: 'Avisos del proyecto',
  programId: 'PROGRAM-ID',
  lines: 'líneas',
  paragraphs: 'párrafos',
  dataItems: 'datos',
  viewer: 'Visor de código',
  noFile: 'Elige un archivo o un resultado de búsqueda para ver su código.',
  search: 'Buscar',
  searchLabel: 'Símbolo o texto',
  searchPlaceholder: 'p. ej. SALDO',
  kind: 'Tipo',
  'kind.all': 'Todos',
  'kind.data': 'Dato',
  'kind.paragraph': 'Párrafo',
  'kind.section': 'Sección',
  'kind.file': 'Archivo',
  'kind.program': 'Programa',
  symbols: 'Símbolos',
  textHits: 'Coincidencias de texto',
  noResults: 'Sin resultados.',
  definedAt: 'Definido en',
  'ref.write': 'Escritura',
  'ref.read': 'Lectura',
  'ref.io': 'E/S',
  'warn.missing_copy': 'COPY no encontrado',
  'warn.dynamic_call': 'CALL dinámico',
  'warn.unresolved_call': 'CALL sin resolver',
  'warn.alter': 'Sentencia ALTER',
  'warn.goto_depending': 'GO TO DEPENDING',
  'warn.not_cobol': 'No parece COBOL',
  'warn.free_format': 'Fuente en formato libre',
  'warn.unsupported': 'Construcción no soportada',
  'err.0': 'No se puede contactar con el servidor.',
  'err.400': 'Solicitud no válida.',
  'err.404': 'No encontrado.',
  'err.413': 'Carga demasiado grande (límite: 1 MB por archivo, 10 MB por proyecto).',
  'err.other': 'Error inesperado.',
  projectGone: 'El proyecto de la URL ya no existe.',
}

export const dict: Record<Lang, Record<Key, string>> = { en, es }

export function detectLang(): Lang {
  try {
    const s = localStorage.getItem('lang')
    if (s === 'en' || s === 'es') return s
  } catch {
    /* storage unavailable */
  }
  return navigator.language.toLowerCase().startsWith('es') ? 'es' : 'en'
}

export const I18nContext = createContext<Lang>('en')
export function useT() {
  const lang = useContext(I18nContext)
  return (k: string) => (dict[lang] as Record<string, string>)[k] ?? k
}
