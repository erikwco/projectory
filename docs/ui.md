# Navegación por proyectos

La TUI sirve para encontrar un proyecto y entrar a una de sus carpetas. Conserva la tipografía del terminal, el fondo de Ghostty y el acento turquesa de la selección. La interfaz se adapta a filas y columnas, sin animaciones.

## Lista y detalle

`list` y `dash` muestran una entrada por ID de proyecto. La fila contiene el nombre y un círculo `⚬` por ubicación, hasta tres. Los círculos usan el acento turquesa y se separan con un espacio; `⚬ ⚬ ⚬ +` indica más de tres. Todos los proyectos disponibles terminan en `›`: Enter entra directamente si hay una ubicación o abre el selector si hay varias. El conteo exacto, las etiquetas, las ramas y las rutas pertenecen al detalle y al selector. Las líneas discretas separan proyectos; en terminales de poca altura se omiten para conservar filas útiles. Tanto los círculos como las líneas discretas responden a preferencias del usuario.

A partir de 110 columnas y 18 filas, el detalle aparece dentro de un recuadro a la derecha, de hasta 64 columnas. En paneles estrechos, la lista ocupa el ancho completo y Tab abre el detalle desplazable.

Por debajo de 24 columnas u 8 filas, un aviso sustituye cualquier vista, incluidos los formularios y las confirmaciones. `q` o Ctrl+C permiten salir; los formularios abiertos directamente también aceptan Esc para cancelar. Las demás teclas no ejecutan acciones ocultas. Al ampliar el terminal, vuelve la vista que estaba abierta.

El proyecto muestra visitas acumuladas y la última visita entre sus ubicaciones. La búsqueda compara los datos de cada ubicación y presenta su proyecto una sola vez. Conserva todas sus rutas y preselecciona una coincidencia en el selector.

`/` enfoca la búsqueda y la lista se filtra mientras se escribe. ↑/↓ aplica el filtro, devuelve el foco a la lista y mueve la selección en la misma pulsación; Enter después activa el proyecto. Sin usar flechas, el primer Enter o Tab solo aplica el filtro y devuelve el foco a la lista con el primer resultado seleccionado; ya en la lista, Tab abre el detalle. Esc durante la edición recupera la búsqueda aplicada anteriormente.

## Formularios

Crear y editar comparten una columna de campos con etiquetas encima de sus valores. En terminales de al menos 60 columnas y 18 filas, un recuadro discreto centra el formulario y limita su ancho a 80 columnas. En espacios menores desaparece el recuadro para conservar el área de edición. Se mantiene el fondo del terminal y la paleta de la lista.

El campo activo usa una fila turquesa, una etiqueta destacada y el indicador `>`. Los valores existentes conservan el color de texto del terminal; las sugerencias y etiquetas inactivas usan el color secundario. Los campos obligatorios llevan `*`. El orden de campos y las teclas de edición se conservan.

El pie permanece fijo al cambiar de campo o mostrar un error. Contiene el progreso, la navegación con Tab/Shift+Tab cuando hay espacio y las acciones Ctrl+S y Esc. Al reducir la altura, se muestran menos campos; el activo siempre queda visible y los contadores ↑/↓ indican cuántos quedan fuera de la vista. Las rutas largas se desplazan horizontalmente siguiendo el cursor, incluso después de cambiar el tamaño.

Guardar con un campo obligatorio vacío lo enfoca, destaca su etiqueta y muestra qué falta. El error desaparece al corregirlo; durante el guardado desde el dashboard aparece «Guardando…». La vista mínima conserva el campo activo y las acciones; por debajo de 24 × 8 se bloquea la edición hasta recuperar espacio.

## Ajustes

F2 abre una pantalla de ajustes desde la lista o la búsqueda. `pt settings` abre la misma pantalla directamente. Conserva el fondo y la tipografía del terminal, la selección turquesa y el pie con las teclas disponibles. El interruptor usa `[x] Activado` o `[ ] Desactivado`, de modo que su estado no depende solo del color. En terminales estrechos conserva la casilla junto al nombre.

↑/↓ o Tab eligen entre **Buscar al abrir** y **Orden predeterminado**. La fila seleccionada tiene el indicador `>` y el fondo turquesa. Espacio o Enter cambian su valor. Para el orden, ←/→ recorren registro, última visita, visitas y nombre en ambos sentidos; debajo se explica el criterio elegido. En ventanas estrechas, el nombre del ajuste y su valor ocupan dos filas para que ambos sigan visibles.

**Buscar al abrir** inicia `pt`, `list` y `dash` con el buscador enfocado. Escribir, pegar o borrar desde la lista retoma la búsqueda actual; conserva la primera letra y vuelve a filtrar. Todas las letras son texto, incluidas q/j/k. Ctrl+C sale; en la lista, Ctrl+O cambia el orden y Ctrl+R actualiza. En el dashboard, Ctrl+A/E/D abren las acciones de registro, edición y eliminación. El selector de ubicaciones y los formularios conservan sus teclas. Desactivado conserva el inicio con foco en la lista y los atajos de letras. Espacio o Enter cambian el interruptor; se confirma el nuevo valor solo cuando termina el guardado. Un error conserva el valor anterior y permite reintentar.

Esc o F2 cierran ajustes y restauran la vista, el filtro y el foco anteriores. Ambas preferencias afectan a la próxima apertura, sin cambiar el orden, el foco ni los atajos de la sesión actual. `--sort` tiene prioridad sobre el orden guardado; o/Ctrl+O solo modifica la sesión. La pantalla ocupa la terminal como los formularios existentes; no tapa la lista con un diálogo. Los avisos de tamaño mínimo también bloquean cambios en ajustes.

## Elección de ubicación

Enter sobre un proyecto con una ruta entra directamente. Con varias rutas abre un selector centrado con etiquetas, ramas y rutas. Las flechas eligen la ubicación y Enter ejecuta la acción. Esc cierra el selector y conserva la búsqueda y el proyecto seleccionado. Tab abre el detalle de la ubicación elegida.

En el dashboard, editar o quitar un proyecto con varias rutas abre el mismo selector para elegir un destino explícito. Quitar siempre confirma la ubicación y conserva sus archivos.

Se compararon dos estructuras antes de implementar:

- Expandir las rutas bajo la fila del proyecto. Mantiene todo en la lista, pero altera las posiciones al navegar y vuelve a introducir filas de ubicaciones entre proyectos.
- Abrir un selector al activar el proyecto. Mantiene estable la lista, concentra el foco en la elección y usa el mismo flujo en paneles amplios y estrechos. Se eligió esta estructura, propuesta también por el usuario.

## Estado y datos

`projects.go` deriva grupos de ubicaciones del catálogo, sin cambiar SQLite. Cada grupo conserva IDs, visitas y disponibilidad de todas sus ubicaciones. El orden por visitas usa la suma del proyecto; los órdenes por registro y última visita usan la fecha más reciente del grupo.

`Browser` conserva por separado la selección del proyecto y la selección de la ubicación. Un proyecto con varias rutas no tiene un destino implícito para navegación o administración. Las recargas conservan las selecciones por ID; si se elimina la última ubicación, se cierra el selector y se vuelve al proyecto vecino.

Se descartó agrupar únicamente durante el renderizado porque dejaría la navegación y las acciones vinculadas a índices de ubicaciones ocultas. El grupo explícito permite usar el mismo modelo en la lista, el detalle y el selector.

Los comandos de texto y JSON siguen entregando ubicaciones individuales. La selección final devuelve la ubicación concreta al shell, que cambia de carpeta y registra su visita.
