# Requisitos — historias de usuario y criterios de aceptación

## HU-01 Cargar archivos COBOL

Como desarrollador, quiero cargar uno o varios fuentes y copybooks para analizarlos juntos.

- Acepta `.cbl`, `.cob`, `.cpy` y `.txt`, con arrastrar y soltar o con el selector. Límite de 1 MB
  por archivo y de 10 MB por proyecto, validado en el servidor.
- Un botón «Usar ejemplos» / «Use samples» carga los fuentes de muestra.
- Cada carga crea un proyecto en el servidor (SQLite) y devuelve su id, que queda en la URL para recargar la página.
- Si un archivo no parece COBOL (no tiene ninguna DIVISION), se avisa pero se deja cargado.
- Cada archivo muestra su resumen: el PROGRAM-ID, el número de líneas, de párrafos y de datos, y los avisos.

## HU-02 Buscar por símbolo

Como desarrollador, quiero buscar un nombre (p. ej. `SALDO`) y ver dónde se define y se usa.

- La búsqueda no distingue mayúsculas, es por subcadena y se filtra por tipo: dato, párrafo, sección, archivo o programa.
- Cada resultado muestra su definición (`archivo:línea`) y sus referencias, clasificadas en
  **escritura** (MOVE … TO, COMPUTE, ADD/SUBTRACT/MULTIPLY/DIVIDE … GIVING/TO, READ INTO, ACCEPT),
  **lectura** y **E/S** (OPEN/READ/WRITE/REWRITE/DELETE/CLOSE).
- Al pulsar una referencia, el visor salta a la línea y la resalta.
- Los nombres que vienen de un COPY resuelto citan el copybook, no el programa.

## HU-03 Preguntar en lenguaje natural

Como desarrollador, quiero preguntar «¿Dónde se calcula el saldo y qué archivos intervienen?» y recibir
una explicación que pueda verificar.

- La pregunta se puede hacer en inglés o en español, y la respuesta llega en el idioma de la UI.
- El usuario elige el proveedor (OpenAI, Claude o Gemini) entre los que tengan clave en el servidor.
- La respuesta cita cada afirmación con `archivo:línea` o `archivo:línea-línea`, y cada cita es un enlace al visor.
- Antes de devolver la respuesta, el servidor comprueba que cada cita exista y caiga dentro de un
  fragmento que se envió como contexto. Una cita inválida se elimina y se lista como «cita no verificada».
- La respuesta enseña qué fragmentos se usaron como evidencia, en un panel desplegable.
- Si no hay evidencia suficiente, responde «no encontrado en los archivos cargados» en lugar de inventar.
- Sin ningún proveedor configurado, cae en un **modo sin IA**: devuelve solo la evidencia (símbolos,
  escrituras y archivos de E/S de los párrafos implicados), sin redactar una explicación.

## HU-04 Ver el mapa de llamadas

Como desarrollador, quiero ver un grafo de qué párrafos llaman a cuáles y qué programas externos se invocan.

- Nodos: párrafos y secciones, y programas externos (CALL).
- Aristas: `PERFORM X`, `PERFORM X THRU Y` (se expande al rango), `GO TO`, `CALL 'LIT'`.
- Al pulsar un nodo se abre su definición en el visor.
- Desde un resultado de búsqueda o una respuesta se puede resaltar el subgrafo relevante.
- **Mapa parcial**: aparece la etiqueta «Mapa parcial» / «Partial map» y la lista de motivos cuando existe alguno de estos casos:
  - `CALL identificador` (nombre dinámico): nodo «? (dinámico)» con la línea de origen.
  - `COPY` sin el copybook cargado: aviso con el nombre que falta.
  - `CALL 'X'` cuando `X` no está cargado: el nodo externo aparece con borde discontinuo.
  - `ALTER` o `GO TO … DEPENDING ON`: la arista se marca como incierta.

## HU-05 Idioma de la interfaz

Como usuario, quiero usar la app en inglés o en español.

- Hay un selector EN/ES. La primera vez se toma el idioma del navegador y después se recuerda la elección.
- Todos los textos de la UI salen del diccionario; ninguno está escrito directamente en los componentes.

## Requisitos no funcionales

- Indexar un fuente de 2 000 líneas tarda menos de 1 s y responder una pregunta menos de 15 s.
- Solo los fragmentos seleccionados salen hacia el proveedor LLM, y la UI lo indica.
- Las claves de API solo existen en las variables de entorno del servidor y nunca aparecen en el repositorio.
- Se despliega con un `docker run`, con los datos en un volumen.
- Accesible: navegable con teclado y con contraste AA.
- El parser está cubierto con pruebas sobre los ejemplos.
