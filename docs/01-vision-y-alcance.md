# Legacy Code Navigator — Visión y alcance

## Problema

Un desarrollador que tiene que modificar un programa COBOL que no conoce pierde horas
leyendo miles de líneas en formato fijo para responder preguntas básicas: dónde se calcula
un valor, qué archivos lee o escribe, qué párrafos llaman a cuáles. El riesgo no es solo el
tiempo: es tocar algo sin ver qué más depende de ello.

## Propuesta

Un explorador web donde el usuario carga uno o varios fuentes COBOL y CL de IBM i (y sus copybooks) y:

1. Busca por símbolo (dato, párrafo, sección, archivo, programa llamado).
2. Pregunta en lenguaje natural («¿Dónde se calcula el saldo y qué archivos intervienen?»)
   y recibe una explicación **con citas `archivo:línea`** verificables.
3. Ve un mapa del flujo de llamadas (PERFORM / CALL) que se marca como **parcial** cuando
   hay dependencias que no se pueden resolver. El mapa escala a sistemas grandes con tres vistas
   (sistema, programa y foco) y un tope de aristas.

## Usuario objetivo

Desarrollador (no necesariamente experto en COBOL) que va a mantener o migrar un programa
y necesita entenderlo antes de tocarlo.

## Alcance

| Incluido | Fuera de alcance |
|---|---|
| Carga de archivos `.cbl`, `.cob`, `.cpy` y `.txt`, en formato fijo o libre | Conexión a AS/400, mainframe o repositorios reales |
| **IBM i CL**: programas `.clp`, `.clle` y `.cl` (o `.txt` con `PGM`/`ENDPGM`), que son la entrada del batch: `CALL`, `SBMJOB`, `TFRCTL`, `OVRDBF` y `GOTO` entran en el mapa y en la búsqueda | DDS (los campos de los archivos de `DCLF`), RPG y comandos propios (`*CMD`) |
| Preguntas con Claude, OpenAI, Gemini u OpenRouter (a elegir) | Autenticación de usuarios |
| UI en inglés y español | Otros idiomas |
| Despliegue con Docker | Kubernetes, alta disponibilidad |
| Índice de símbolos: divisiones, secciones, párrafos, datos (niveles 01–49, 77, 88), archivos (SELECT/FD), COPY, CALL | Dialectos con extensiones propietarias |
| `COPY … REPLACING` (pseudotexto, palabra, `LEADING`/`TRAILING`) y varios programas por archivo (consecutivos o anidados) | Nombres `GLOBAL` del programa padre visibles en un hijo anidado |
| Clasificación de todos los verbos COBOL para lecturas y escrituras | |
| Búsqueda por símbolo con ubicación y referencias (definición, lecturas, escrituras) | Ejecución, depuración o compilación del COBOL |
| Pregunta en lenguaje natural con respuesta citada; con proveedor, la pregunta se asigna antes a símbolos del índice | Edición de código o refactorización asistida |
| Grafo de llamadas PERFORM (interno) y CALL (externo), con vistas sistema, programa y foco (tope de 300 aristas) | Análisis de JCL, CICS, DB2 embebido (`EXEC SQL` se muestra como bloque opaco) |
| Aviso de mapa parcial con lista de lo no resuelto | Cuotas por usuario y límite de peticiones |
| Ejemplos incluidos en el repo: uno pequeño (4 archivos) y una demo grande (12 programas COBOL, 1 programa CL, 8 copybooks, 3 102 líneas) | |
| Los proyectos caducan solos (`PROJECT_TTL`, 7 días por defecto) | |

## Criterios de éxito

- Con los ejemplos incluidos, la pregunta «¿Dónde se calcula el saldo y qué archivos
  intervienen?» devuelve las sentencias correctas y los archivos correctos, con citas que
  apuntan a líneas reales.
- **Ninguna cita inventada**: toda cita `archivo:línea` mostrada existe y se valida contra el
  fuente antes de enseñarla.
- El mapa nunca se presenta como completo si hay un CALL dinámico, un copybook ausente o un
  `GO TO`/`ALTER` sin resolver.
- Un fuente de unas 2 000 líneas se indexa en menos de 1 s.
- `docker run` levanta la app completa con los ejemplos disponibles.
- La demo grande (`?samples=large`) se navega con el mapa acotado: ninguna vista supera 300 aristas.

## Qué demuestra (portafolio)

Análisis estático de código, recuperación de información sobre un índice propio,
respuestas de LLM ancladas en evidencia, diseño pensado para sistemas legacy y trazabilidad
de punta a punta (pregunta → símbolos → líneas → respuesta).
