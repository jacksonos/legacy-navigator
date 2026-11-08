# Legacy Code Navigator — Visión y alcance

## Problema

Un desarrollador que tiene que modificar un programa COBOL que no conoce pierde horas
leyendo miles de líneas en formato fijo para responder preguntas básicas: dónde se calcula
un valor, qué archivos lee o escribe, qué párrafos llaman a cuáles. El riesgo no es solo el
tiempo: es tocar algo sin ver qué más depende de ello.

## Propuesta

Un explorador web donde el usuario carga uno o varios fuentes COBOL (y sus copybooks) y:

1. Busca por símbolo (dato, párrafo, sección, archivo, programa llamado).
2. Pregunta en lenguaje natural («¿Dónde se calcula el saldo y qué archivos intervienen?»)
   y recibe una explicación **con citas `archivo:línea`** verificables.
3. Ve un mapa del flujo de llamadas (PERFORM / CALL) que se marca como **parcial** cuando
   hay dependencias que no se pueden resolver.

## Usuario objetivo

Desarrollador (no necesariamente experto en COBOL) que va a mantener o migrar un programa
y necesita entenderlo antes de tocarlo.

## Alcance del MVP

| Incluido | Fuera de alcance |
|---|---|
| Carga de archivos `.cbl`, `.cob`, `.cpy` (formato fijo) | Conexión a AS/400, mainframe o repositorios reales |
| Preguntas con OpenAI, Claude o Gemini (a elegir) | Autenticación de usuarios |
| UI en inglés y español | Otros idiomas |
| Despliegue con Docker | Kubernetes, alta disponibilidad |
| Índice de símbolos: divisiones, secciones, párrafos, datos (niveles 01–49, 77, 88), archivos (SELECT/FD), COPY, CALL | Formato libre COBOL 2002+, dialectos con extensiones propietarias |
| Búsqueda por símbolo con ubicación y referencias (definición, lecturas, escrituras) | Ejecución, depuración o compilación del COBOL |
| Pregunta en lenguaje natural con respuesta citada | Edición de código o refactorización asistida |
| Grafo de llamadas PERFORM (interno) y CALL (externo) | Análisis de JCL, CICS, DB2 embebido (`EXEC SQL` se muestra como bloque opaco) |
| Aviso de mapa parcial con lista de lo no resuelto | |
| Archivos COBOL de ejemplo incluidos en el repo | |

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

## Qué demuestra (portafolio)

Análisis estático de código, recuperación de información sobre un índice propio,
respuestas de LLM ancladas en evidencia, diseño pensado para sistemas legacy y trazabilidad
de punta a punta (pregunta → símbolos → líneas → respuesta).
