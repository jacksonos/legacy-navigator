# Plan de desarrollo

## Estructura del repositorio

```
server/      módulo Go: main.go, api.go, store.go, cobol/, llm/, samples/ (embebidos)
web/         React + TS + Vite
docs/        estos documentos
Dockerfile   docker-compose.yml   .env.example
```

## Archivos de ejemplo (`server/samples/`)

Escritos a medida, con nombres y comentarios en español:

| Archivo | Contenido | Para qué sirve |
|---|---|---|
| `CALCSALD.cbl` | Lee `MOVIMIENTOS` y `CUENTAS`, calcula el saldo (COMPUTE/ADD/SUBTRACT) y reescribe `CUENTAS`; escribe `REPORTE` | Pregunta canónica, E/S y PERFORM THRU |
| `CTAREG.cpy` | Registro de la cuenta (`CTA-SALDO`, niveles 88) | Expansión de COPY y citas a un copybook |
| `MOVREG.cpy` | Registro de movimiento | Ídem |
| `VALIDAR.cbl` | Subprograma invocado con `CALL 'VALIDAR'` | CALL resuelto |
| Dentro de `CALCSALD` | `CALL WS-RUTINA` y `COPY AUDITREG` (sin el archivo) | Disparar el **mapa parcial** a propósito |

## Hitos

| Hito | Entregable | Hecho cuando |
|---|---|---|
| H0 Base | Esqueleto Go y Vite, `samples/`, Dockerfile y `.gitignore` | `go test ./...`, `npm run build` y `docker build` en verde; `GET /` sirve la app embebida |
| H1 Parser | `cobol/`: normalizador, expansor de COPY y parser | Pruebas por tabla sobre los ejemplos: los párrafos, datos, referencias, aristas y avisos esperados, con sus líneas exactas |
| H2 Almacén y búsqueda | SQLite + FTS5, endpoints de proyectos, archivos y búsqueda; UI de carga, visor y búsqueda; i18n (HU-01, HU-02, HU-05) | Buscar `SALDO` lista la definición y las escrituras correctas y salta a la línea, en EN y en ES |
| H3 Grafo | `/graph` y Mermaid con la etiqueta «Mapa parcial» (HU-04) | Con los ejemplos aparecen los 2 motivos de mapa parcial |
| H4 Preguntas | Recuperación, `llm/` con los 3 proveedores, validador y modo sin IA (HU-03) | La pregunta canónica devuelve citas válidas con al menos un proveedor real; una prueba del validador elimina una cita falsa |
| H5 Pulido | README con capturas, `docker-compose.yml` y limitaciones visibles en la UI | `docker compose up` deja la app funcionando con los ejemplos |

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
- `main` siempre en verde. Cada hito va en una rama `feat/hN-<tema>` y entra en `main` con un PR.
- No se versionan `.env`, `data/`, `web/dist`, `node_modules` ni binarios.
