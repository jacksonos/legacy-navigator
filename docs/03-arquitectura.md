# Arquitectura y decisiones técnicas

## Vista general

```
┌──────────────────────┐   JSON/HTTP   ┌─────────────────────────────────────────┐
│ web/ React+TS+Vite   │ ────────────► │ server/ Go (net/http)                   │
│  carga · búsqueda    │               │  cobol/  normaliza · COPY · parsea       │
│  visor · grafo       │ ◄──────────── │  cl/     parsea CL de IBM i              │
│  (Mermaid) · i18n    │               │  SQLite + FTS5  (símbolos, refs, líneas) │
└──────────────────────┘               │  ask     recupera → LLM → valida citas   │
                                       │  llm/    anthropic | openai | gemini |   │
                                       │          openrouter                      │
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
| LLM | Anthropic, OpenAI, Gemini y OpenRouter por HTTP directo | Una interfaz `Provider` con cuatro proveedores (OpenRouter reutiliza el cliente de OpenAI), sin SDK |
| Pruebas | `go test` | Parser, almacén, grafo, validador, recuperador y proveedores. Sin pruebas unitarias del front |
| CI | GitHub Actions | Jobs `server` (gofmt, vet, test), `web` (lint, build) y `docker` (build de la imagen) |
| Despliegue | Docker (imagen multi-etapa) | Ver «Despliegue» |

## API

| Método y ruta | Qué hace |
|---|---|
| `POST /api/projects` | Sube los archivos (multipart; `.cbl`, `.cob`, `.cpy`, `.txt`, `.clp`, `.clle`, `.cl`) y devuelve `{id, resumen, avisos}`. Con `?samples=1` carga los ejemplos pequeños y con `?samples=large` la demo grande |
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
type FileSummary = { name: string; lines: number; language: "cobol" | "cl" | "copybook"; programId: string /* el primero */; programIds: string[] /* todos los del archivo */; paragraphs: number; dataItems: number; warnings: Warning[] }
type Project = { id: string; files: FileSummary[]; warnings: Warning[] }      // warnings = nivel proyecto

// POST /api/projects (multipart, campo "files" repetido) | POST /api/projects?samples=1|large   → 201 Project
// GET  /api/projects/{id}                                                                → 200 Project
// GET  /api/projects/{id}/files/{name}      → 200 { name: string; lines: string[] }      // líneas crudas, índice 0 = línea 1
// GET  /api/projects/{id}/search?q=&kind=   kind ∈ data|paragraph|section|file|program (opcional)
type Ref = { kind: "read" | "write" | "io"; file: string; line: number; paragraph: string; program: string; detail?: string }   // program = PROGRAM-ID que contiene la línea; detail = dato extra (CL: destino de OVRDBF, `LIB/ARCHIVO`)
type SymbolHit = { name: string; kind: string; program: string /* "" si viene de un copybook */; def: { file: string; line: number }; refs: Ref[] }
type LineHit = { file: string; line: number; text: string }                    // aciertos FTS5 sobre las líneas
// → 200 { results: SymbolHit[]; lines: LineHit[] }   (máx. 50 de cada)

// Errores: { error: string } con 400 / 404 / 413
```

Un archivo puede contener varios programas (consecutivos con `END PROGRAM` o anidados). Los nodos del grafo
ya usan `PROG:PÁRRAFO`; `program` en `Ref`, `SymbolHit` y `Evidence` permite pedir la vista Foco sin
deducir el programa a partir del archivo.

### Contrato del grafo (H3)

```ts
// GET /api/projects/{id}/graph → 200 Graph
type GraphNode = {
  id: string                      // estable y único: "PROG:PARRAFO", "PROG" (programa), "ext:NOMBRE", "dyn:archivo:línea"
  label: string
  kind: "program" | "section" | "paragraph" | "external" | "dynamic"
  language: "cobol" | "cl" | ""   // lenguaje del programa que contiene el nodo; "" en external y dynamic
  file: string; line: number      // vacío/0 para external y dynamic sin origen
  resolved: boolean               // false: CALL a programa no cargado o dinámico → borde discontinuo
}
type GraphEdge = { from: string; to: string; kind: "perform" | "goto" | "call"; file: string; line: number; uncertain: boolean; async: boolean }
// async = true solo en SBMJOB (un `call` que corre en otro trabajo). En CL, CALL, TFRCTL y SBMJOB son aristas `call`
type Graph = { nodes: GraphNode[]; edges: GraphEdge[]; partial: boolean; reasons: Warning[] }
// partial = hay algún aviso missing_copy | dynamic_call | unresolved_call | alter | goto_depending; reasons = esos avisos
```

#### Vistas del grafo (escalabilidad)

El grafo completo de un sistema real (cientos de programas, miles de párrafos) no se puede leer ni dibujar.
El servidor devuelve siempre una **vista** acotada:

```ts
// GET /api/projects/{id}/graph?view=system|program|focus&program=PROG&node=ID&depth=1..3
type Graph = {
  view: "system" | "program" | "focus"   // la vista realmente servida (puede diferir de la pedida)
  program: string                        // programa de la vista program/focus ("" en system)
  node: string                           // nodo central en focus ("" en otras)
  depth: number                          // profundidad servida en focus (0 en otras)
  programs: string[]                     // todos los PROGRAM-ID cargados, para el selector
  nodes: GraphNode[]; edges: GraphEdge[]; partial: boolean; reasons: Warning[]
}
```

- **system**: un nodo por programa + `ext:`/`dyn:`; aristas `call` agregadas programa → destino. Vista por
  defecto con más de un programa; con uno solo se sirve `program`.
- **program**: secciones/párrafos de `program` + destinos de sus CALL (programa, `ext:` o `dyn:`).
- **focus**: vecinos a `depth` saltos (sin dirección) de `node` sobre perform/goto/call.
- **Tope** `maxGraphEdges = 300`: si `program` lo supera se sirve `focus` sobre su primer párrafo con
  depth 2; si `focus` lo supera se reduce `depth`, y con depth 1 se recortan aristas. Todo recorte añade a
  `reasons` un aviso `{kind:"too_large", detail:"<mostradas>/<total>"}` y pone `partial: true`.
- `reasons` en program/focus solo incluye los avisos de ese programa; en system, todos.
- Parámetros inválidos (vista, programa o nodo inexistente, depth fuera de 1..3) → 400.

### Contrato de preguntas (H4)

```ts
// GET /api/providers → 200 { providers: { id: "openai" | "anthropic" | "gemini" | "openrouter"; model: string }[] }   // solo los configurados

// POST /api/projects/{id}/ask
type AskRequest = { question: string /* 1..500 caracteres */; lang: "en" | "es"; provider?: string }
type Evidence = { file: string; startLine: number; endLine: number; paragraph: string; program: string; text: string }  // text = líneas crudas unidas con \n
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

### Contrato de CL (IBM i)

Un archivo `.clp`, `.clle` o `.cl` (o un `.txt` con `PGM` y `ENDPGM`) es un programa CL. Se analiza con el paquete
`cl/` y entra en el mismo índice que COBOL:

- **Programa**: el id es el nombre del archivo sin extensión, en mayúsculas (el nombre del objeto en IBM i). Las etiquetas
  `LABEL:` son bloques (`paragraph` en `symbols` y en el grafo); las sentencias previas a la primera etiqueta pertenecen
  al programa (las aristas salen del nodo del programa y, como evidencia, forman un bloque con el nombre del programa).
- **Símbolos**: variables (`&NOMBRE`, con el `&`), etiquetas, archivos declarados con `DCLF`.
- **Referencias**: `CHGVAR` y `RTV*` escriben `&X`; el resto de usos de `&X` leen. `OVRDBF`, `DLTOVR`, `DCLF`, `RCVF` y
  `SNDRCVF` son `io` sobre el nombre del archivo; en `OVRDBF` el `detail` es el destino (`LIB/ARCHIVO`).
- **Aristas**: `CALL`, `CALLPRC`, `TFRCTL` y `SBMJOB CMD(CALL ...)` son `call` (biblioteca quitada del destino);
  `SBMJOB` con `async: true`. `GOTO` y `MONMSG ... EXEC(GOTO ...)` son `goto` entre etiquetas.
- **Avisos**: `CALL PGM(&VAR)` es `dynamic_call`; un destino no cargado es `unresolved_call`. Un CALL entre CL y COBOL
  se resuelve por nombre en los dos sentidos.
- **Almacén**: `programs.language` (`cobol` | `cl`), `refs.detail` y `edges.async`, con `ALTER TABLE ... ADD COLUMN`
  protegido para que las bases antiguas sigan funcionando. El `language` de un archivo se deduce de sus programas;
  sin programas y sin aviso `not_cobol` es `copybook`.

## Módulos del servidor

1. **`cobol/`** (paquete puro, sin E/S; es el núcleo que se prueba):
   - **Normalizador**: aplica las reglas del formato fijo. Ignora las columnas 1–6 y descarta todo
     desde la columna 73. La columna 7 marca el tipo de línea: `*` o `/` es comentario, `-` es
     continuación y `D` es depuración. `>>SOURCE FORMAT FREE`, `>>SOURCE FREE` y
     `$SET SOURCEFORMAT"FREE"` (sin distinguir mayúsculas) pasan a formato libre desde esa línea:
     sin columnas, y los párrafos deben empezar con sangría menor que 4. `>>SOURCE FORMAT FIXED`
     vuelve al fijo. `*>` abre un comentario hasta el fin de línea en ambos formatos (fuera de
     literales). Cada fragmento conserva su `(archivo, línea)` original, **base de toda la trazabilidad**.
   - **Expansor de COPY**: sustituye `COPY X` por el copybook cargado y conserva el origen de cada
     línea. Si el copybook falta, emite un aviso de mapa parcial. `COPY X REPLACING` admite
     pseudotexto `==a b== BY ==c==` (puede abarcar varias líneas), la forma de palabra `A BY B` y
     `LEADING`/`TRAILING` (reemplazo parcial dentro de un identificador). Se aplica a los tokens
     del copybook ya expandido y cada token resultante conserva el `Loc` del copybook. El `COPY`
     recursivo se omite con un aviso `unsupported`.
   - **Parser**: parte cada archivo en programas (consecutivos con `END PROGRAM x.` y anidados:
     cada uno es un `Program` con su archivo y su rango de líneas) y separa las sentencias por
     punto y por verbo. Produce `Program { id, sections, paragraphs, data, files, refs, edges,
     warnings }`; párrafos, secciones y datos pertenecen al programa que los contiene y un `CALL` a
     un programa contenido se resuelve. Cada referencia tiene un tipo:
     `def | read | write | io | perform | call | goto`. La tabla de verbos cubre todos los verbos
     COBOL: cada uno abre un modo de operandos nuevo (lectura por defecto) y los terminadores
     `END-*` y las frases `WHEN`, `ELSE`, `NOT`, `AT END`, `INVALID KEY`, `ON SIZE ERROR` lo
     reinician. Los destinos de escritura incluyen `INSPECT`, `SET`, `SEARCH … VARYING`,
     `RETURN … INTO`, `DIVIDE … REMAINDER`, `STRING`/`UNSTRING … POINTER|TALLYING`,
     `CALL … RETURNING` y `ACCEPT`. En un subíndice o una modificación de referencia, el nombre base
     toma el modo de la sentencia y los identificadores entre paréntesis son lecturas.
2. **`cl/`** (paquete puro, como `cobol/`): `Detect(nombre, contenido)` decide si un archivo es CL (extensión, o `.txt` con
   `PGM` y `ENDPGM`; es el único sitio donde se decide el lenguaje) y `Analyze` devuelve un `cobol.Project`: reutiliza los
   tipos `Program`, `Block`, `Ref`, `Edge` y `Warning` de `cobol/` (con los campos nuevos `Program.Language`, `Ref.Detail` y
   `Edge.Async`), así que almacén, grafo, búsqueda y preguntas no distinguen lenguajes salvo para mostrarlo. El lexer une las
   líneas (comentarios `/* */` multilínea, continuación `+` que quita los blancos iniciales y `-` que los conserva), separa
   etiquetas, comandos y parámetros `KWD(valor)` y analiza de forma recursiva los comandos anidados (`SBMJOB CMD(…)`,
   `IF … THEN(…)`, `ELSE`, `MONMSG … EXEC(…)`). `server/analyze.go` reparte los archivos entre `cobol.Analyze` y `cl.Analyze`
   y resuelve los `CALL` por nombre entre los programas de los dos lenguajes (sin ello un `CALL` de COBOL a un programa CL
   saldría como `unresolved_call`).
3. **Almacén (SQLite)**: guarda el resultado del parser.
   - Tablas: `projects`, `files(project, name, content)`, `symbols`, `refs(…, detail)`, `edges(…, async)`, `warnings`, `programs(project, name, file, start_line, end_line, language)`. El programa de una línea (`program` en `Ref`, `SymbolHit` y `Evidence`) se deduce del rango más interno que la contiene; sin fila en `programs` (proyectos antiguos) se cae a las líneas `PROGRAM-ID` de `symbols`.
   - `lines_fts` es una tabla virtual FTS5 con `(project UNINDEXED, file UNINDEXED, line UNINDEXED, text)`.
     Incluye los comentarios, que a menudo dicen «calcula saldo».
   - El tokenizador `unicode61` parte los nombres por el guion: `WS-SALDO-FINAL` se encuentra
     buscando `saldo`.
4. **Recuperación (`ask`)**:
   1. La pregunta se reduce a términos (sin palabras vacías en/es) y se buscan con FTS5 `MATCH`
      sobre las líneas y con `LIKE` sobre los nombres de símbolo.
   2. Se reúnen los párrafos que contienen esos aciertos, los párrafos que **escriben** los símbolos
      encontrados, los archivos con E/S en esos párrafos y sus vecinos a un salto en el grafo.
   3. Con todo ello se arman fragmentos de párrafos completos con números de línea, con un tope
      configurable (`ASK_MAX_CHARS`).
   4. Solo con proveedor, antes del paso 1 se hace una llamada previa (15 s de tope) que asigna la
      pregunta a nombres de símbolo del proyecto (datos, párrafos, secciones, archivos, programas;
      máx. 3000). Solo se conservan los nombres que existen en el índice, y se suman a los aciertos
      por término como símbolos acertados. Si la llamada falla, se sigue solo con los términos; sin
      proveedor no hay expansión.
5. **`llm/`**: la interfaz `Provider { Complete(ctx, system, user) (string, error) }` con cuatro
   proveedores: Anthropic, OpenAI, Gemini y OpenRouter (este último con el cliente de OpenAI). Se activa
   cada proveedor que tenga clave **y** modelo (`FromEnv`); solo Anthropic tiene un modelo por defecto.
6. **Validador de citas**: extrae cada `[archivo:línea]` o `[archivo:línea-línea]` y comprueba que
   exista y que caiga dentro de un fragmento enviado. Las citas inválidas pasan a `unverified` y se
   quitan del texto.
7. **Modo sin IA**: si no hay ningún proveedor configurado, `/ask` devuelve solo la evidencia
   (símbolos, escrituras y archivos de E/S) sin redactar una explicación.

## Configuración (entorno)

| Variable | Uso |
|---|---|
| `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `OPENROUTER_API_KEY` | Activan el proveedor correspondiente (junto con su modelo) |
| `ANTHROPIC_MODEL`, `OPENAI_MODEL`, `GEMINI_MODEL`, `OPENROUTER_MODEL` | Modelo por proveedor. Por defecto `claude-sonnet-5-5` solo para Anthropic; los otros tres son obligatorios (sin ellos el proveedor se desactiva con un aviso en el log). OpenRouter usa ids con prefijo, p. ej. `openai/gpt-4o` |
| `ASK_MAX_CHARS` | Máximo de caracteres de código enviados al LLM como evidencia (por defecto 24000) |
| `DB_PATH` | Ruta del archivo SQLite (por defecto `./data/navigator.db`) |
| `ADDR` | Dirección de escucha (por defecto `:8080`) |
| `PROJECT_TTL` | Vida de un proyecto (duración Go, por defecto `168h`; `0` = no borrar). Limpieza al arrancar y cada hora |

## Despliegue (Docker)

`Dockerfile` multi-etapa en la raíz:

1. `node:22-alpine` ejecuta `npm ci && npm run build` en `web/` y produce `web/dist`.
2. `golang` copia `web/dist` en `server/` para `//go:embed` y compila con `CGO_ENABLED=0 go build`.
3. `gcr.io/distroless/static-debian12:nonroot` recibe solo el binario. Corre como usuario no root, expone el puerto
   8080 y monta un volumen en `/data`, con `DB_PATH=/data/navigator.db`.

```
docker build -t legacy-navigator .
docker run -p 8080:8080 -v navigator-data:/data -e ANTHROPIC_API_KEY=... legacy-navigator
```

Además, un `docker-compose.yml` mínimo para levantar todo en local con un `.env` opcional. El `.env` no
se versiona; el repo incluye un `.env.example`.

**Dependencias del front**: `mermaid` está fijado a `11.17.2` y `katex` (dependencia transitiva de Mermaid) se
fuerza a `^0.18.2` con `overrides` en `web/package.json` para corregir un aviso de seguridad; los `$` de las
etiquetas del mapa se escapan para que KaTeX no los interprete.

## Decisiones

| # | Decisión | Alternativa descartada | Por qué |
|---|---|---|---|
| D1 | Parser y almacén en Go, en el servidor | Analizar en el navegador | La búsqueda con FTS5 vive en el servidor; además, un solo parser probado con `go test` |
| D2 | Parser con regex por sentencia | ANTLR o una gramática COBOL completa | Basta para la estructura y las referencias; lo que no se entiende sale como aviso, nunca se ignora en silencio |
| D3 | Búsqueda con FTS5 sobre líneas y símbolos | Embeddings o RAG vectorial | El código tiene identificadores exactos; FTS5 es determinista, explicable y no requiere servicios extra |
| D4 | Citas validadas por código | Confiar en el LLM | La trazabilidad es el punto del proyecto |
| D5 | HTTP directo a los cuatro proveedores (OpenRouter con el cliente de OpenAI) | SDKs oficiales | Solo hace falta una llamada de texto; ninguna dependencia por proveedor |
| D8 | Expansión de la pregunta a símbolos con el LLM, validada contra el índice | Confiar en los nombres que devuelve el LLM | Cierra la brecha de vocabulario («balance» → `WS-SALDO`) sin perder determinismo: solo cuentan los nombres que existen |
| D9 | Vistas acotadas del grafo (sistema, programa, foco) con tope de 300 aristas | Dibujar el grafo completo | Un grafo de miles de nodos no se lee ni se dibuja; el recorte siempre se declara como mapa parcial |
| D6 | Un binario con el front embebido | Imágenes separadas para front y back | Una sola imagen, sin CORS ni proxy |
| D7 | `modernc.org/sqlite` | `mattn/go-sqlite3` | Sin cgo: compila igual en Windows, Linux y Docker distroless |
| D10 | CL como un segundo analizador (`cl/`) que produce los mismos tipos que `cobol/`; el lenguaje se guarda en `programs.language` | Un modelo de datos propio para CL, o deducir el lenguaje de la extensión en cada consulta | En IBM i los CL son las entradas del batch (`SBMJOB`/`CALL` a los COBOL, `OVRDBF` que enlaza los nombres lógicos con los archivos físicos): reutilizar los tipos deja el almacén, el grafo y `ask` sin ramas por lenguaje. Una columna con `ALTER TABLE … ADD COLUMN` protegido es más robusta que la extensión (un `.txt` puede ser CL) y no rompe las bases antiguas |

## Limitaciones conocidas (se documentan en la UI)

Son las mismas nueve que muestra el botón «Limitaciones» (`limit.1` a `limit.9` en `web/src/i18n.ts`):

1. Se admiten el formato libre y `COPY … REPLACING`; en formato libre, los párrafos deben empezar con sangría menor que 4. En `REPLACING` el pseudotexto se compara por palabras completas (no hay el idioma `:TAG:` parcial).
2. En modo solo evidencia (sin proveedor), los términos de la pregunta deben aparecer en el código o sus comentarios (p. ej. «balance» no encuentra `WS-SALDO`); con proveedor, la pregunta se asigna antes a símbolos.
3. Los nombres calificados (`X OF Y`) y `REDEFINES` se resuelven solo por nombre; dos datos con el mismo nombre en grupos distintos se muestran ambos.
4. Los bloques `EXEC SQL` / `EXEC CICS` son opacos: solo se extraen las variables host.
5. Los programas anidados se analizan por separado: los nombres `GLOBAL` del programa padre no son visibles en el hijo.
6. El mapa es parcial cuando no se pueden resolver llamadas o copybooks (y cuando una vista se recorta por el tope de aristas).
7. Solo los fragmentos de código seleccionados se envían al proveedor de IA.
8. Los proyectos se borran automáticamente a los 7 días (configurable con `PROJECT_TTL`). No hay cuotas por usuario ni límite de peticiones.
9. CL (IBM i): los comandos se leen como texto, no se ejecutan. No se siguen los comandos propios (`*CMD`), los procedimientos de `CALLPRC` ni los campos de los archivos de `DCLF` (DDS); los `CALL` se resuelven por nombre y sin tener en cuenta la biblioteca.
