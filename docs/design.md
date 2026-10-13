# Modelo e integración con el shell

Projectory separa el proyecto que una persona reconoce de las carpetas que contienen su código. Una copia antigua y un worktree pueden pertenecer al mismo proyecto sin compartir repositorio Git.

## Identidad de proyectos y ubicaciones

Un proyecto tiene un ID, un nombre, una descripción y tags. Una ubicación tiene otro ID, una ruta absoluta, una etiqueta y sus propios contadores de visitas. Cambiar la ruta mediante `edit` conserva el ID y las visitas.

La relación con Git pertenece a cada ubicación. Los worktrees vinculados comparten el directorio Git común, pero mantienen su rama y sus archivos de trabajo. Los clones independientes tienen directorios Git diferentes aunque compartan un remoto.

La agrupación automática usa el directorio Git común de las raíces registradas. Una subcarpeta de un monorepo no reclama todas las ubicaciones del repositorio. `--project` permite agrupar las copias explícitamente.

Las tecnologías se detectan por ubicación. Así una copia antigua puede conservar un stack distinto del que usa la carpeta principal.

## Propiedad de los datos

`internal/catalog` define los datos compartidos, la normalización de rutas, los filtros y los órdenes. No depende de la terminal ni de SQLite.

`internal/store` controla el esquema SQLite y las transacciones. La deduplicación y la agrupación por repositorio se resuelven bajo el bloqueo de escritura para que dos terminales no creen registros distintos para la misma ubicación.

`internal/inspect` lee los manifiestos e invoca el Git instalado. Usa los formatos de salida preparados para programas, incluidos separadores NUL para las rutas de worktrees. Cada proceso Git tiene un tiempo límite.

`internal/app` coordina registro, edición y actualización. Tanto los comandos como los formularios utilizan esas operaciones.

`internal/tui` contiene los estados de navegación, búsqueda, confirmación y formularios. Las consultas de Git y las escrituras se ejecutan fuera del manejo inmediato de teclas. Los resultados se entregan a Bubble Tea como mensajes.

La TUI deriva una fila por proyecto y conserva sus ubicaciones dentro del grupo. Cuando hay varias rutas, la navegación y la administración requieren elegir una ubicación explícita. Los contadores del proyecto suman sus visitas, pero la navegación sigue registrando la visita de la ubicación concreta. La [guía de interfaz](ui.md) describe la agrupación, el selector y las decisiones visuales.

`internal/cli` define los comandos y sus salidas. `internal/shell` genera las funciones para zsh y bash.

## Preferencias de navegación

`internal/config.Settings` contiene `SearchOnOpen` y `DefaultSort`. `Load(path)` devuelve los valores predeterminados si el archivo no existe; una configuración anterior sin orden conserva `added`. La lectura y la escritura rechazan órdenes desconocidos. `Save(path, values)` escribe un archivo temporal en la misma carpeta y lo renombra para evitar lecturas parciales. La configuración es global al usuario y no depende del catálogo seleccionado.

El comando de listado carga las preferencias una vez y resuelve el orden: `--sort` explícito, orden guardado o `added`. Esa decisión se comparte entre TUI, texto y JSON; la misma configuración se entrega al navegador. `SettingsScreen` controla la selección del ajuste, el guardado asíncrono y su resultado. Solo actualiza el valor visible después de un guardado exitoso. `Browser` conserva el estado al que debe volver y fija `AutoSearch` al crear la sesión: decide el foco inicial y si escribir desde la lista retoma la búsqueda. Cambiar las preferencias durante la sesión conserva sus atajos y su orden actual. Las flechas verticales confirman el filtro y delegan el movimiento al mismo estado de listado; las horizontales siguen editando el texto. Enter y Tab confirman el filtro y seleccionan el primer resultado sin reenviar la tecla, de modo que Tab no abre el detalle hasta la siguiente pulsación. El comando `settings` reutiliza la misma pantalla o guarda los valores indicados mediante `--search-on-open` y `--default-sort`.

Se consideró guardar preferencias en SQLite, pero cambiar de catálogo cambiaría también el comportamiento del teclado. El archivo de configuración del usuario comparte las preferencias entre todos los catálogos. Sin archivo se conserva el comportamiento anterior; los scripts de prueba y de capturas aíslan la configuración mediante `PROJECTORY_CONFIG_DIR`.

## Cambio de directorio

El directorio de trabajo pertenece a cada proceso. El binario puede cambiar el suyo, pero no el del shell que lo ejecutó. Por eso `pt` es una función del shell que llama a `projectory`.

El atajo `pt ~` se resuelve en la función, antes de llamar al binario. Reconoce un único argumento: el carácter `~` literal o la ruta personal ya expandida por el shell. Ejecuta `builtin cd` sin argumentos, conserva su código de salida y no abre SQLite ni registra una visita. Las funciones cargadas antes de una actualización requieren ejecutar de nuevo `init` para incorporar cambios en esta integración.

Para navegar, la función crea un archivo temporal privado y ejecuta el selector. El binario escribe el ID de la ubicación, la ruta y el archivo del catálogo separados por NUL. La interfaz se dibuja en la terminal por separado.

La función lee esos campos como datos y ejecuta `builtin cd` con la ruta entre comillas. No evalúa nombres ni rutas como código. El catálogo viaja en el mismo resultado para que un `--db` relativo siga apuntando al archivo correcto después del cambio de directorio.

Después de un `cd` exitoso, la función pide al binario que incremente la visita. Si SQLite falla en ese momento, la navegación permanece y el shell muestra que no pudo registrar la visita. No existe una transacción atómica que abarque a la vez el directorio del shell y SQLite.

Cancelar el selector produce una selección vacía. La función conserva el directorio actual y elimina su archivo temporal.

## Catálogo local y actualizaciones

SQLite guarda los datos dentro del directorio del usuario. Las transacciones usan bloqueos de escritura inmediatos y una espera limitada. WAL permite leer mientras otra terminal escribe.

El estado de disponibilidad se consulta al leer el catálogo. Una carpeta ausente conserva sus metadatos y sus visitas. Al quitar una ubicación explícitamente, se guarda una exclusión para evitar que el descubrimiento de worktrees la agregue otra vez.

El registro de un proyecto y las actualizaciones son operaciones explícitas. La primera versión no necesita un proceso permanente, un servicio de red ni un explorador de todo el disco.

## Verificación

Los tests de Go crean repositorios temporales para comprobar worktrees en carpetas separadas, clones independientes, estados sin rama y proyectos anidados. Los tests de SQLite comprueban persistencia, concurrencia, reversión de ediciones y respaldos.

`scripts/smoke.py` ejecuta el binario real en zsh, bash y pseudoterminales. Comprueba la secuencia de búsqueda, el cambio de tamaño, el cambio de directorio del shell, las visitas y los formularios. Las rutas de prueba incluyen espacios, comillas, caracteres de shell y saltos de línea.

## Distribución y actualización del ejecutable

Los tags de versión generan cuatro archivos precompilados mediante GoReleaser: macOS y Linux, cada uno en amd64 y arm64. Los archivos tienen nombres estables y un manifiesto SHA-256 compartido por el actualizador y la fórmula de Homebrew. Los candidatos `-rc.N` requieren una selección explícita.

`internal/update` consulta las releases del repositorio oficial. La CLI recibe la versión incorporada por el compilador y separa la consulta de la instalación. Una instalación independiente descarga y verifica el archivo, valida el ejecutable y lo renombra sobre el destino desde un temporal en la misma carpeta. Conserva enlaces simbólicos y permisos. Las rutas administradas por Homebrew remiten a `brew upgrade`; las compilaciones de desarrollo se actualizan recompilando.

La actualización no depende de `internal/app` ni abre el catálogo o los ajustes. La navegación normal tampoco depende de la red. Las pruebas de errores de descarga y validación exigen que el ejecutable anterior se conserve.

`internal/shell.ExecutablePath` conserva la ruta de invocación si corresponde al ejecutable actual. En una instalación de Homebrew prefiere la ruta estable `opt/projectory/bin/projectory`, comprobada con `os.SameFile`. Así las funciones `pt` ya cargadas sobreviven a `brew upgrade` y a la limpieza de versiones anteriores.
