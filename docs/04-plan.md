# Plan de desarrollo

## Estructura del repositorio

```
server/               módulo Go: main.go, api.go, askapi.go, ask.go, cite.go, graph.go, store.go
  cobol/              normalizador, expansor de COPY y parser
  cl/                 parser de CL de IBM i (.clp, .clle, .cl)
  llm/                proveedores Anthropic, OpenAI, Gemini y OpenRouter
  samples/            ejemplos pequeños (embebidos)
  samples-large/      demo bancaria grande (embebida)
web/                  React + TS + Vite
docs/                 estos documentos y screenshots/
.github/workflows/    ci.yml (GitHub Actions)
Dockerfile   docker-compose.yml   .env.example
```

## Archivos de ejemplo pequeños (`server/samples/`)

Escritos a medida, con nombres y comentarios en español:

| Archivo | Contenido | Para qué sirve |
|---|---|---|
| `CALCSALD.cbl` | Lee `MOVIMIENTOS` y `CUENTAS`, calcula el saldo (COMPUTE/ADD/SUBTRACT) y reescribe `CUENTAS`; escribe `REPORTE` | Pregunta canónica, E/S y PERFORM THRU |
| `CTAREG.cpy` | Registro de la cuenta (`CTA-SALDO`, niveles 88) | Expansión de COPY y citas a un copybook |
| `MOVREG.cpy` | Registro de movimiento | Ídem |
| `VALIDAR.cbl` | Subprograma invocado con `CALL 'VALIDAR'` | CALL resuelto |
| Dentro de `CALCSALD` | `CALL WS-RUTINA` y `COPY AUDITREG` (sin el archivo) | Disparar el **mapa parcial** a propósito |

## Demo grande (`server/samples-large/`)

Un sistema bancario de 12 programas COBOL (`BATCHDIA`, `CARGAMOV`, `CIERRE`, `COMISION`, `ERRORES`, `EXTRACTO`, `FECHAS`,
`INTERESES`, `MORAS`, `SALDOS`, `VALIDCTA`, `AUDITLOG`), el trabajo nocturno CL `CIERREDIA.clp` (entrada del batch: `OVRDBF`,
`CALL` a `BATCHDIA` y `SBMJOB` de `EXTRACTO`), 8 copybooks y 3 102 líneas. Se carga con «Demo grande» o con
`POST /api/projects?samples=large`. Incluye un `CALL` dinámico, un `COPY` ausente, un `CALL` a un programa no cargado y un
`GO TO … DEPENDING ON`, y sirve para probar las vistas del mapa y la recuperación sobre un proyecto de tamaño real.

## Hitos

| Hito | Entregable | Hecho cuando |
|---|---|---|
| H0 Base | Esqueleto Go y Vite, `samples/`, Dockerfile y `.gitignore` | `go test ./...`, `npm run build` y `docker build` en verde; `GET /` sirve la app embebida |
| H1 Parser | `cobol/`: normalizador, expansor de COPY y parser | Pruebas por tabla sobre los ejemplos: los párrafos, datos, referencias, aristas y avisos esperados, con sus líneas exactas |
| H2 Almacén y búsqueda | SQLite + FTS5, endpoints de proyectos, archivos y búsqueda; UI de carga, visor y búsqueda; i18n (HU-01, HU-02, HU-05) | Buscar `SALDO` lista la definición y las escrituras correctas y salta a la línea, en EN y en ES |
| H3 Grafo | `/graph` y Mermaid con la etiqueta «Mapa parcial» (HU-04) | Con los ejemplos aparecen los 2 motivos de mapa parcial |
| H4 Preguntas | Recuperación, `llm/` con los 3 proveedores de entonces (OpenRouter llegó después, PR #8), validador y modo sin IA (HU-03) | La pregunta canónica devuelve citas válidas con al menos un proveedor real; una prueba del validador elimina una cita falsa |
| H5 Pulido | README con capturas, `docker-compose.yml` y limitaciones visibles en la UI | `docker compose up` deja la app funcionando con los ejemplos |

## Evolución post-MVP

| PR | Cambio |
|---|---|
| #7 | CI con GitHub Actions: jobs `server` (gofmt, vet, test), `web` (lint, build) y `docker` |
| #8 | Proveedor OpenRouter (`OPENROUTER_API_KEY` + `OPENROUTER_MODEL`), con el cliente de OpenAI y la cabecera `X-Title` |
| #9 | Las declaraciones `SELECT` / `FD` entran como evidencia en las preguntas sobre archivos |
| #10 | Caducidad de proyectos: `PROJECT_TTL` (168 h por defecto, `0` la desactiva) y limpieza cada hora |
| #11 | Expansión pregunta → símbolos con el LLM antes de recuperar (≤ 15 nombres validados contra el índice, 15 s de tope, con respaldo) |
| #12 | Vistas escalables del grafo (sistema, programa, foco), tope de 300 aristas con motivo `too_large` y selección de nodo con «Open code» / «Focus here» |
| #13 | Demo grande en `server/samples-large/` (12 programas, 8 copybooks, 3 045 líneas), `?samples=large` y botón «Large demo» |
| #14 | Parser: formato libre, `COPY … REPLACING`, varios programas por archivo (tabla `programs`, `programIds`, `program` en `Ref`/`SymbolHit`/`Evidence`) y tabla completa de verbos COBOL |
| #15 | «Ver en el mapa» se oculta en la evidencia de `FILE-CONTROL` / `FILE SECTION` |
| #16 | Aviso de seguridad de `katex` corregido con `overrides` (`^0.18.2`, Mermaid sigue en 11.17.2) y `$` escapado en las etiquetas del mapa |
| #18 (esta rama) | Soporte de IBM i CL: paquete `cl/`, `.clp`/`.clle`/`.cl`, `programs.language`, `refs.detail` y `edges.async`, aristas `SBMJOB` asíncronas, nodos CL en el mapa y `CIERREDIA.clp` en la demo grande |

## Estado actual

Todas las historias de usuario (HU-01 a HU-05) están hechas y la CI está en verde en `main`.

## Próximos pasos (opcionales)

- Demo pública en un VPS (Debian 13 recomendado) con Caddy para HTTPS y la imagen publicada en GHCR.
- Límite de peticiones en `POST /api/projects/{id}/ask`.
- Tope de gasto en OpenRouter para la clave de la demo.

## Estrategia de pruebas

- **Parser** (`go test`, por tabla): símbolo → líneas de definición, escritura y aristas esperadas. Es la prueba que más importa.
- **Validador de citas**: casos de cita válida, línea fuera de rango, archivo inexistente y línea fuera del contexto.
- **Recuperación**: sobre un SQLite en memoria con los ejemplos, comprobar que «saldo» trae los párrafos que escriben `CTA-SALDO`.
- **Proveedores LLM**: una prueba por proveedor contra un `httptest.Server` que valida la forma de
  la petición y la lectura de la respuesta. Las llamadas reales no forman parte de la suite.
- **Prueba de humo de punta a punta** en el navegador, en modo sin IA, al final de H4.

## Git y GitHub

- Remoto: `https://github.com/jacksonos/legacy-navigator.git`.
- Commits con Conventional Commits (`feat(cobol): …`, `docs: …`), pequeños y con un cambio lógico
  cada uno. Sin coautoría de IA.
- `main` siempre en verde. Cada hito va en una rama `feat/hN-<tema>` y entra en `main` con un PR; los cambios posteriores
  usan ramas `feat/…`, `fix/…` o `docs/…`, también con PR.
- No se versionan `.env`, `data/`, `web/dist`, `node_modules` ni binarios.
