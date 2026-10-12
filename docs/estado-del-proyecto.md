# Estado del proyecto

Projectory está en la versión `0.1.0-dev` y sigue en desarrollo. Todavía no hay versiones publicadas. Este documento describe la implementación y sus límites.

## Objetivo y preferencias

Projectory reúne proyectos guardados en carpetas diferentes y permite cambiar de directorio desde un selector de terminal.

Las prioridades acordadas son:

- Una utilidad local en Go con comandos y pantallas TUI.
- Registro de la carpeta actual o de una ruta introducida en un formulario.
- Nombre y descripción opcional, con categorías mediante tags.
- Listado con una entrada por proyecto, ordenado por registros recientes. La búsqueda empieza con `/` y filtra mientras se escribe. Enter devuelve el foco al listado. Otro Enter abre el proyecto si tiene una ruta o pide elegir una ubicación si tiene varias. Esc durante la búsqueda recupera el filtro anterior.
- Dashboard con administración y contadores de visitas.
- SQLite para persistencia y detección automática de tecnologías.
- Copias independientes y worktrees creados por distintas herramientas, incluso en carpetas alejadas entre sí.
- Uso en terminales con ventanas y monitores de tamaños distintos. La interfaz está en español.
- Código abierto bajo la licencia MIT.

`pr` fue un nombre provisional. Se eligió `projectory` para el ejecutable y `pt` como nombre predeterminado de la función del shell. Ese nombre se puede cambiar con `init zsh --cmd NOMBRE`.

## Repositorio

La ruta del módulo Go es `github.com/erikwco/projectory`. El código fuente, los tests y la documentación se versionan juntos. El binario local, los catálogos, los ajustes personales y las capturas de revisión quedan fuera de Git.

La licencia es [MIT](../LICENSE). `main` solo acepta cambios mediante pull request; el flujo está en [CONTRIBUTING.md](../CONTRIBUTING.md). Los reportes de seguridad siguen [SECURITY.md](../SECURITY.md).

## Implementación disponible

La versión del ejecutable es `0.1.0-dev`. El código usa Go 1.26 y declara `toolchain go1.26.9` en `go.mod`. Las dependencias directas están fijadas en ese archivo: Bubble Tea y Bubbles v2, Lip Gloss v2, Cobra, `modernc.org/sqlite`, `golang.org/x/term` y las utilidades ANSI de Charm.

Los comandos implementados son `add`, `list`, `dash`, `go`, `edit`, `remove`, `refresh`, `backup`, `info`, `settings` e `init`. Sin subcomando se abre el selector. `_visit` y `--selection-file` son parte interna de la comunicación con el shell.

El registro distingue proyectos y ubicaciones. Nombre, descripción y tags se comparten por proyecto; ruta, etiqueta, tecnologías y visitas pertenecen a cada ubicación. La corrección de una ruta conserva la identidad y el contador de visitas.

Los worktrees se relacionan mediante el directorio Git común, no por su ubicación física ni solo por el remoto. Las copias independientes se asocian explícitamente con `--project`. Registrar una subcarpeta de un monorepo conserva esa ruta y no importa los worktrees de todo el repositorio.

La interfaz filtra mientras se escribe, se pega o se borra texto.

F2 o `pt settings` permiten activar **Buscar al abrir**. La preferencia se guarda fuera del catálogo y se aplica en la siguiente apertura de `pt` o `dash`. En ese modo, escribir siempre filtra, también después de moverse por la lista. ↑/↓ transfiere el foco y mueve la selección; Enter después abre el proyecto. Sin flechas se conserva la secuencia de dos Enter. Las letras se reservan para el filtro; Ctrl+A/E/D administran desde el dashboard, Ctrl+R actualiza, Ctrl+O ordena y Ctrl+C sale. Sin configuración previa se conserva el inicio manual con `/` y los atajos de letras.

En la misma pantalla, **Orden predeterminado** permite guardar registro, última visita, visitas o nombre para próximas aperturas. Se aplica a TUI, texto y JSON; `--sort` explícito tiene prioridad. Cambiar el orden con o/Ctrl+O afecta solo a la sesión actual.

En `list` y `dash`, cada proyecto aparece una sola vez, con círculos discretos que indican sus ubicaciones. Se eliminó el sufijo `/ principal` del listado; el conteo exacto, las etiquetas, las ramas y las rutas están en el detalle y en el selector de ubicaciones. Enter abre un selector cuando hay varias rutas, incluso si solo una coincide con la búsqueda. Editar y quitar también requieren elegir una ubicación concreta cuando hay varias.

Las líneas discretas separan proyectos y el fondo de color destaca la selección. El detalle está enmarcado a la derecha en ventanas amplias; en paneles estrechos, Tab abre una vista desplazable. La [guía de interfaz](ui.md) conserva las decisiones de agrupación y navegación.

Los formularios de creación y edición usan una columna centrada, un campo activo resaltado y un pie fijo con las acciones. En terminales pequeñas muestran menos campos y conservan visible el cursor, incluso en rutas largas. Los errores de campos obligatorios enfocan el campo pendiente y desaparecen al corregirlo.

Quitar un registro conserva los archivos. Un worktree quitado queda excluido de `refresh` hasta que se registre explícitamente otra vez. Las carpetas desaparecidas permanecen en el catálogo como no disponibles.

La integración con zsh y bash confirma el cambio de carpeta antes de sumar la visita. Solo cuenta entradas realizadas mediante la función `pt`. Un `cd` manual no aumenta el contador.

`pt ~` vuelve a la carpeta personal sin abrir el catálogo ni contar una visita. `pt` sin argumentos conserva el selector. Este atajo requiere recargar la función del shell en terminales abiertas antes de actualizar.

## Mapa de archivos

- `cmd/projectory/main.go`: entrada del ejecutable y versión.
- `internal/catalog/`: tipos, normalización de rutas, búsqueda y orden.
- `internal/config/`: preferencias persistentes del usuario, independientes del catálogo.
- `internal/store/`: SQLite, esquema, transacciones, deduplicación, visitas y respaldos.
- `internal/inspect/`: manifiestos, descubrimiento de worktrees y estado Git.
- `internal/app/`: registro, edición y actualización compartidos por CLI y TUI.
- `internal/tui/`: formularios, selector, dashboard, ajustes, detalle y estilos.
- `internal/cli/`: comandos, opciones y salidas de texto o JSON.
- `internal/shell/`: generación de la función `pt` para zsh y bash.
- `scripts/smoke.py`: pruebas del ejecutable real y de pseudoterminales.
- `scripts/demo.py`: grabación de los GIF del README con datos ficticios; se ejecuta con `make demos`.
- `scripts/preview.py`: capturas de revisión de la TUI real con datos temporales mediante VHS.
- `scripts/check_public_files.py`: rechaza catálogos, credenciales, binarios y capturas de revisión versionados.
- `Makefile`: compilación, instalación y verificaciones.
- `.github/workflows/ci.yml`: CI con `make check` en Linux y macOS y el job Secrets.
- `docs/commands.md`: referencia de comandos y teclas.
- `docs/design.md`: decisiones de modelo, persistencia e integración.
- `docs/demos.md`: generación de los GIF de `docs/media/`.

El binario compilado para el entorno local está en `bin/projectory`. `bin/` está excluido de Git y se regenera con `make build`.

## Instalación, función `pt` y datos

`make install` copia el ejecutable a `~/.local/bin/projectory`. `PREFIX` permite cambiar el prefijo de instalación. No modifica `.zshrc` ni crea un archivo `pt`.

La instrucción `eval "$("$HOME/.local/bin/projectory" init zsh)"` define `pt` en la sesión que la ejecuta. La misma línea en `.zshrc` vuelve a definir la función en cada nueva sesión interactiva de zsh. La función guarda la ruta absoluta del ejecutable que generó la definición.

Si `pt` apunta al binario instalado, mover el código fuente no rompe esa referencia. Si se activó con `./bin/projectory`, se debe regenerar la función después de trasladar la carpeta.

El catálogo queda fuera del código fuente. En macOS, la ruta predeterminada es `~/Library/Application Support/projectory/catalog.db`. `--db`, `PROJECTORY_DATA_DIR` y `XDG_DATA_HOME` pueden cambiarla. `pt info` muestra la ruta efectiva.

Las pruebas de desarrollo usan catálogos temporales. El catálogo del usuario queda fuera del código fuente.

## Verificación realizada

`make check` se ha verificado localmente en macOS arm64. Ejecuta:

- `go test -race ./...`.
- Compilación del ejecutable.
- `python3 scripts/smoke.py`.
- `go vet ./...`.

Las comprobaciones cubren persistencia, escrituras concurrentes, deduplicación, respaldos, edición atómica, worktrees externos, clones independientes, proyectos anidados, detección de tecnologías, búsqueda, cambio de tamaño y formularios.

Las pruebas en pseudoterminales verificaron el recorrido completo de selección, `cd` en el shell padre y contador de visitas. Se ejecutaron en zsh y bash, con rutas que incluyen espacios, comillas, caracteres de shell y saltos de línea. También cubren la búsqueda de un worktree externo, su elección dentro del proyecto y la eliminación de una ubicación sin afectar las otras ni sus archivos.

La agrupación por proyectos, los ajustes, el foco de búsqueda y los formularios se verifican con tests y capturas de terminal. El workflow [Checks](https://github.com/erikwco/projectory/actions/workflows/ci.yml) ejecuta `make check` en Linux y macOS y revisa secretos y archivos privados en el job Secrets. Consulta cada ejecución para conocer el resultado de un commit concreto.

## Límites actuales y próximos pasos

`refresh` revisa repositorios conocidos; no explora todo el disco. El estado respecto al upstream usa referencias locales, sin `fetch` automático. La aplicación no crea, borra ni repara worktrees Git.

La primera versión incluye salida JSON, pero no importador JSON ni sincronización entre equipos. La integración de navegación cubre zsh y bash. La detección de tecnologías lee los manifiestos de la carpeta registrada, sin recorrer recursivamente todo un monorepo.

Las pruebas automatizadas cubren cambios de tamaño y navegación. El uso con distintos monitores, niveles de zoom, copias y worktrees reales puede revelar ajustes de comodidad; repórtalos en los issues del repositorio.

La [guía de publicación](publicacion.md) recoge las comprobaciones del mantenedor sobre la visibilidad del repositorio y la preparación de una versión. Las [instrucciones para contribuir](../CONTRIBUTING.md) permiten reproducir las pruebas con datos temporales.
