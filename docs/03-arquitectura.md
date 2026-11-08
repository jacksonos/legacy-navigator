# Arquitectura y decisiones técnicas

## Vista general

```
┌──────────────────────┐   JSON/HTTP   ┌─────────────────────────────────────────┐
│ web/ React+TS+Vite   │ ────────────► │ server/ Go (net/http)                   │
│  carga · búsqueda    │               │  cobol/  normaliza · COPY · parsea       │
│  visor · grafo       │ ◄──────────── │  SQLite + FTS5  (símbolos, refs, líneas) │
│  (Mermaid) · i18n    │               │  ask     recupera → LLM → valida citas   │
└──────────────────────┘               │  llm/    openai | anthropic | gemini     │
                                       └─────────────────────────────────────────┘
        Un único binario Go sirve la API y el build de web/ (embed) → una imagen Docker
```

## Stack

| Pieza | Elección | Nota |
|---|---|---|
| Front | React + TypeScript + Vite | Sin librería de estado ni de router: una sola pantalla |
| Grafo | Mermaid (`flowchart`) | El servidor devuelve nodos y aristas, el front genera el texto Mermaid |
| i18n | Diccionario `en`/`es` propio (un objeto por idioma) | Sin librería; la respuesta del LLM se pide en el idioma de la UI |
| Back | Go, solo `net/http` | Rutas con método y comodines de `ServeMux` (Go 1.22 o posterior), sin framework |
| Base de datos | SQLite con `modernc.org/sqlite` | Go puro (sin cgo) y con FTS5 incluido, lo que permite compilar estático para Docker y Windows |
| LLM | OpenAI, Anthropic y Gemini por HTTP directo | Una interfaz `Provider` con tres implementaciones, sin SDK |
| Pruebas | `go test` | Parser, validador y recuperador. Sin pruebas unitarias del front en el MVP |
| Despliegue | Docker (imagen multi-etapa) | Ver «Despliegue» |

## API

| Método y ruta | Qué hace |
|---|---|
| `POST /api/projects` | Sube los archivos (multipart) y devuelve `{id, resumen, avisos}`. Con `?samples=1` carga los ejemplos |
| `GET /api/projects/{id}` | Resumen por archivo y avisos de mapa parcial |
| `GET /api/projects/{id}/files/{name}` | Fuente con números de línea |
| `GET /api/projects/{id}/search?q=&kind=` | Símbolos con su definición y sus referencias clasificadas |
| `GET /api/projects/{id}/graph` | Nodos, aristas (`perform`, `goto`, `call`, con la marca `uncertain`) y motivos de mapa parcial |
| `POST /api/projects/{id}/ask` | `{question, lang, provider}` → `{answer, citations[], evidence[], unverified[]}` |
| `GET /api/providers` | Proveedores configurados, para el selector de la UI |

Las claves de los LLM solo viven en el servidor, en variables de entorno. El navegador nunca las envía.

### Contratos JSON (H2)

```ts
type Warning = { kind: string; detail: string; file: string; line: number }   // file/line pueden ir vacíos/0
type FileSummary = { name: string; lines: number; programId: string; paragraphs: number; dataItems: number; warnings: Warning[] }
type Project = { id: string; files: FileSummary[]; warnings: Warning[] }      // warnings = nivel proyecto

// POST /api/projects (multipart, campo "files" repetido) | POST /api/projects?samples=1   → 201 Project
// GET  /api/projects/{id}                                                                → 200 Project
// GET  /api/projects/{id}/files/{name}      → 200 { name: string; lines: string[] }      // líneas crudas, índice 0 = línea 1
// GET  /api/projects/{id}/search?q=&kind=   kind ∈ data|paragraph|section|file|program (opcional)
type Ref = { kind: "read" | "write" | "io"; file: string; line: number; paragraph: string }
type SymbolHit = { name: string; kind: string; def: { file: string; line: number }; refs: Ref[] }
type LineHit = { file: string; line: number; text: string }                    // aciertos FTS5 sobre las líneas
// → 200 { results: SymbolHit[]; lines: LineHit[] }   (máx. 50 de cada)

// Errores: { error: string } con 400 / 404 / 413
```

### Contrato del grafo (H3)

```ts
// GET /api/projects/{id}/graph → 200 Graph
type GraphNode = {
  id: string                      // estable y único: "PROG:PARRAFO", "PROG" (programa), "ext:NOMBRE", "dyn:archivo:línea"
  label: string
  kind: "program" | "section" | "paragraph" | "external" | "dynamic"
  file: string; line: number      // vacío/0 para external y dynamic sin origen
  resolved: boolean               // false: CALL a programa no cargado o dinámico → borde discontinuo
}
type GraphEdge = { from: string; to: string; kind: "perform" | "goto" | "call"; file: string; line: number; uncertain: boolean }
type Graph = { nodes: GraphNode[]; edges: GraphEdge[]; partial: boolean; reasons: Warning[] }
// partial = hay algún aviso missing_copy | dynamic_call | unresolved_call | alter | goto_depending; reasons = esos avisos
```

### Contrato de preguntas (H4)

```ts
// GET /api/providers → 200 { providers: { id: "openai" | "anthropic" | "gemini" | "openrouter"; model: string }[] }   // solo los configurados

// POST /api/projects/{id}/ask
type AskRequest = { question: string /* 1..500 caracteres */; lang: "en" | "es"; provider?: string }
type Evidence = { file: string; startLine: number; endLine: number; paragraph: string; text: string }  // text = líneas crudas unidas con \n
type Citation = { file: string; line: number; endLine: number }   // endLine = line si es una sola línea
type AskResponse = {
  mode: "llm" | "evidence"        // "evidence" = sin proveedor configurado o sin evidencia (no se llama al LLM)
  provider: string                // "" en modo evidence
  answer: string                  // texto plano; citas [archivo:línea] o [archivo:inicio-fin], todas verificadas
  citations: Citation[]           // las citas válidas que quedaron en answer
  unverified: string[]            // citas eliminadas por inválidas, en crudo
  evidence: Evidence[]            // fragmentos enviados (o que se enviarían) al LLM; [] = no encontrado
}
// Errores { error }: 400 (pregunta vacía/larga, proveedor desconocido o sin configurar), 404, 502 (fallo del proveedor), 504 (timeout 60 s)
```

Sin `provider` en la petición se usa el primero configurado (orden: anthropic, openai, gemini, openrouter).
Un proveedor se activa con su clave **y** su modelo: `ANTHROPIC_MODEL` vale `claude-sonnet-5-5` por defecto;
`OPENAI_MODEL`, `GEMINI_MODEL` y `OPENROUTER_MODEL` son obligatorios para no fijar en el código nombres de modelo que caducan.
OpenRouter expone la API de chat de OpenAI: se reutiliza el mismo cliente con base `https://openrouter.ai/api`, la clave
`OPENROUTER_API_KEY`, modelos con prefijo de proveedor (`openai/…`, `anthropic/…`) y la cabecera `X-Title: Legacy Code Navigator`.

## Módulos del servidor

1. **`cobol/`** (paquete puro, sin E/S; es el núcleo que se prueba):
   - **Normalizador**: aplica las reglas del formato fijo. Ignora las columnas 1–6 y descarta todo
     desde la columna 73. La columna 7 marca el tipo de línea: `*` o `/` es comentario, `-` es
     continuación y `D` es depuración. Cada fragmento conserva su `(archivo, línea)` original,
     **base de toda la trazabilidad**.
   - **Expansor de COPY**: sustituye `COPY X` por el copybook cargado y conserva el origen de cada
     línea. Si el copybook falta, emite un aviso de mapa parcial.
   - **Parser**: separa las sentencias por punto y por verbo, y produce `Program { id, sections,
     paragraphs, data, files, refs, edges, warnings }`. Cada referencia tiene un tipo:
     `def | read | write | io | perform | call | goto`.
2. **Almacén (SQLite)**: guarda el resultado del parser.
   - Tablas: `projects`, `files(project, name, content)`, `symbols`, `refs`, `edges`, `warnings`.
   - `lines_fts` es una tabla virtual FTS5 con `(project UNINDEXED, file UNINDEXED, line UNINDEXED, text)`.
     Incluye los comentarios, que a menudo dicen «calcula saldo».
   - El tokenizador `unicode61` parte los nombres por el guion: `WS-SALDO-FINAL` se encuentra
     buscando `saldo`.
3. **Recuperación (`ask`)**:
   1. La pregunta se reduce a términos (sin palabras vacías en/es) y se buscan con FTS5 `MATCH`
      sobre las líneas y con `LIKE` sobre los nombres de símbolo.
   2. Se reúnen los párrafos que contienen esos aciertos, los párrafos que **escriben** los símbolos
      encontrados, los archivos con E/S en esos párrafos y sus vecinos a un salto en el grafo.
   3. Con todo ello se arman fragmentos de párrafos completos con números de línea, con un tope
      configurable (`ASK_MAX_CHARS`).
4. **`llm/`**: la interfaz `Provider { Complete(ctx, system, user) (string, error) }` con tres
   implementaciones: OpenAI, Anthropic y Gemini. Se activa cada proveedor cuya clave esté presente.
5. **Validador de citas**: extrae cada `[archivo:línea]` o `[archivo:línea-línea]` y comprueba que
   exista y que caiga dentro de un fragmento enviado. Las citas inválidas pasan a `unverified` y se
   quitan del texto.
6. **Modo sin IA**: si no hay ningún proveedor configurado, `/ask` devuelve solo la evidencia
   (símbolos, escrituras y archivos de E/S) sin redactar una explicación.

## Configuración (entorno)

| Variable | Uso |
|---|---|
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY` | Activan el proveedor correspondiente |
| `OPENAI_MODEL`, `ANTHROPIC_MODEL`, `GEMINI_MODEL` | Modelo por proveedor. Por defecto `claude-sonnet-5-5` para Anthropic; los de OpenAI y Gemini se fijan en H4 con la documentación vigente |
| `DB_PATH` | Ruta del archivo SQLite (por defecto `./data/navigator.db`) |
| `ADDR` | Dirección de escucha (por defecto `:8080`) |

## Despliegue (Docker)

`Dockerfile` multi-etapa en la raíz:

1. `node:lts-alpine` ejecuta `npm ci && npm run build` en `web/` y produce `web/dist`.
2. `golang` copia `web/dist` en `server/` para `//go:embed` y compila con `CGO_ENABLED=0 go build`.
3. `gcr.io/distroless/static` recibe solo el binario. Corre como usuario no root, expone el puerto
   8080 y monta un volumen en `/data`, con `DB_PATH=/data/navigator.db`.

```
docker build -t legacy-navigator .
docker run -p 8080:8080 -v navigator-data:/data -e ANTHROPIC_API_KEY=... legacy-navigator
```

Además, un `docker-compose.yml` mínimo para levantar todo en local con un `.env`. Este archivo no
se versiona; el repo incluye un `.env.example`.

## Decisiones

| # | Decisión | Alternativa descartada | Por qué |
|---|---|---|---|
| D1 | Parser y almacén en Go, en el servidor | Analizar en el navegador | La búsqueda con FTS5 vive en el servidor; además, un solo parser probado con `go test` |
| D2 | Parser con regex por sentencia | ANTLR o una gramática COBOL completa | Basta para la estructura y las referencias; lo que no se entiende sale como aviso, nunca se ignora en silencio |
| D3 | Búsqueda con FTS5 sobre líneas y símbolos | Embeddings o RAG vectorial | El código tiene identificadores exactos; FTS5 es determinista, explicable y no requiere servicios extra |
| D4 | Citas validadas por código | Confiar en el LLM | La trazabilidad es el punto del proyecto |
| D5 | HTTP directo a los tres proveedores | SDKs oficiales | Solo hace falta una llamada de texto; tres dependencias menos |
| D6 | Un binario con el front embebido | Imágenes separadas para front y back | Una sola imagen, sin CORS ni proxy |
| D7 | `modernc.org/sqlite` | `mattn/go-sqlite3` | Sin cgo: compila igual en Windows, Linux y Docker distroless |

## Limitaciones conocidas (se documentan en la UI)

- **Términos de la pregunta**: la búsqueda solo encuentra lo que aparece en el código o en sus
  comentarios. «balance» no encuentra `WS-SALDO`.
  `ponytail:` si hace falta, se añade un paso del LLM que traduzca la pregunta a los nombres de símbolo del índice.
- Las referencias calificadas (`X OF Y`) y `REDEFINES` se resuelven por nombre; dos datos con el
  mismo nombre en grupos distintos se muestran ambos.
- `EXEC SQL` / `EXEC CICS`: bloque opaco; solo se extraen las variables host `:VAR` como lecturas.
- `COPY … REPLACING` y el formato libre no están soportados; aparecen como aviso.
- Los proyectos se guardan sin caducidad. `ponytail:` añadir una limpieza por antigüedad si se publica una demo abierta.
