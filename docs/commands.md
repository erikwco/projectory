# Referencia de comandos

`projectory` es el ejecutable. `pt` es la función que genera `projectory init`. Ambos aceptan los mismos subcomandos; la función también cambia el directorio del shell después de una selección y admite el atajo `pt ~`.

## Registro

```sh
pt add --current
pt add
pt add /ruta --name Nombre --description "Descripción" --tags personal,go
pt add /otra/ruta --project Nombre --label experimental
```

`--current` usa la carpeta actual. Sin ruta, `add` solicita una en el formulario. `--name` o `--project` omiten el formulario. `--no-input` usa los valores proporcionados y toma el nombre de la carpeta si no hay uno.

Al registrar una raíz Git, también se registran sus worktrees. `--no-worktrees` limita ese registro a la carpeta indicada. Una actualización posterior con `refresh` descubre los worktrees de las raíces registradas.

Registrar una subcarpeta conserva la ruta exacta y no importa los worktrees de la raíz del repositorio. Los proyectos sin Git también son válidos.

Volver a registrar una ruta existente conserva el nombre, los metadatos y las visitas. `edit` modifica esos valores. Las rutas existentes se normalizan y se resuelven los enlaces simbólicos para evitar duplicados.

## Selector y dashboard

```sh
pt list
pt dash
pt list --query tienda --tag cliente --sort visited
pt list --plain
pt list --json
```

Sin subcomando, `pt` abre el selector. Tanto el selector como el dashboard muestran una entrada por proyecto, con círculos discretos que indican sus ubicaciones. Las etiquetas, las ramas, las rutas y el conteo exacto se muestran en el detalle y al elegir una ubicación.

Los órdenes disponibles son `added` (registro), `visited` (última visita), `visits` (más visitas) y `name` (nombre de A a Z). Sin configuración previa se usa `added`, del registro más reciente al más antiguo. **Orden predeterminado** en F2 cambia ese valor para futuras aperturas. Un `--sort` explícito tiene prioridad sobre el ajuste guardado. En la TUI, `added` y `visited` usan la fecha más reciente entre las ubicaciones del proyecto; `visits` suma sus visitas.

La búsqueda compara palabras sin distinguir mayúsculas y minúsculas contra nombre, ruta, etiqueta, rama, tecnologías y tags. Cada palabra debe aparecer. `--tag` exige una categoría completa. La lista se filtra mientras escribes, pegas o borras texto, tanto en el selector como en el dashboard.

Una coincidencia con una ruta o rama muestra su proyecto una sola vez. El selector de ubicaciones conserva todas sus rutas y preselecciona una coincidencia disponible cuando existe.

`--json` imprime los datos completos. `--plain`, o ejecutar el listado sin una terminal interactiva, imprime una tabla. Ambos conservan una entrada por ubicación y ordenan sus datos individuales. Estos modos no cambian de directorio ni cuentan visitas.

Las teclas del listado con **Buscar al abrir** desactivado son:

- **↑**, **↓**, **j**, **k**: mover la selección.
- **Inicio**, **Fin**, **g**, **G**: ir al principio o al final.
- **RePág**, **AvPág**: desplazar la selección varias filas.
- **/**: editar la búsqueda actual.
- **↑/↓** durante la búsqueda: confirmar el filtro, devolver el foco al listado y mover la selección en la misma pulsación.
- **Enter** durante la búsqueda: confirmar la búsqueda y devolver el foco al listado, sin abrir una carpeta.
- **Enter** en el listado: entrar al proyecto si tiene una ruta; con varias rutas, abrir el selector de ubicaciones.
- **Esc** durante la búsqueda: recuperar el filtro anterior y volver al listado.
- **Esc** en el listado: limpiar el filtro; sin filtro, salir.
- **Tab**: abrir o cerrar el detalle completo.
- **F2** desde el listado o la búsqueda: abrir ajustes.
- **o**: cambiar el orden durante la sesión, sin modificar el predeterminado.
- **r**: actualizar los registros y descubrir worktrees.
- **q**, **Ctrl+C**: salir del listado sin seleccionar.

En el dashboard, **a** abre el registro, **e** abre la edición y **d** solicita quitar la ubicación. **s** o **y** confirman; cualquier otra tecla cancela. Dentro de un campo de texto, las letras se escriben normalmente.

Con **Buscar al abrir** activado, escribir, pegar o borrar desde el listado retoma el filtro actual. Las letras son texto, incluidas **q**, **j** y **k**. Usa las flechas para moverte y **Ctrl+C** para salir. En el listado, **Ctrl+O** cambia el orden y **Ctrl+R** actualiza; en el dashboard, **Ctrl+A/E/D** agregan, editan o quitan. Los demás atajos sin letras se conservan. Dentro del buscador, **Ctrl+A/E** siguen moviendo el cursor de texto.

Con varias rutas, editar y quitar piden elegir la ubicación antes de ejecutar la acción. En el selector de ubicaciones, las flechas cambian la selección, **Enter** ejecuta la acción, **Tab** abre el detalle de esa ubicación y **Esc** vuelve al proyecto sin perder la búsqueda. Allí se conservan **e**, **d** y **q**, independientemente de la preferencia de búsqueda.

En los formularios, **Tab** y **Shift+Tab** cambian de campo. **Enter** avanza y, en el último campo, guarda. **Ctrl+S** guarda desde cualquier campo. **Esc** y **Ctrl+C** cancelan el formulario.

El campo activo y las acciones permanecen visibles al reducir el terminal. Las rutas largas se desplazan con el cursor. Un campo obligatorio vacío se enfoca al intentar guardar; corregirlo retira el aviso. Por debajo de 24 columnas u 8 filas aparece un aviso de tamaño y se bloquea la edición; `q` o Ctrl+C permiten salir.

La interfaz muestra el detalle en un recuadro lateral a partir de 110 columnas y 18 filas. En paneles estrechos, la lista ocupa el ancho disponible y **Tab** permite consultar el detalle. Las líneas separan proyectos; con poca altura se omiten para mostrar más filas. Por debajo de 24 columnas o 8 filas aparece un aviso; **q** sigue permitiendo salir.

## Ajustes de navegación

```sh
pt settings
pt settings --search-on-open=true
pt settings --search-on-open=false
pt settings --default-sort=name
```

`settings` abre los ajustes en una terminal interactiva. Sin terminal, muestra los valores actuales y la ruta del archivo. Los flags modifican las preferencias sin abrir la TUI ni el catálogo; se pueden combinar y conservan los valores no indicados.

**Buscar al abrir** enfoca el buscador al iniciar `pt`, `pt list` o `pt dash`. Puedes escribir, pegar o borrar para filtrar sin pulsar **/**, también después de recorrer la lista. **↑/↓** confirma el filtro y mueve la selección; el siguiente **Enter** activa el proyecto. Sin usar una flecha, el primer **Enter** devuelve el foco al listado y el siguiente activa el proyecto. Desactivado conserva el inicio y los atajos habituales; es el valor predeterminado cuando no hay un archivo de ajustes.

**Orden predeterminado** elige el orden inicial del selector, el dashboard y las salidas de texto o JSON. Los cuatro valores son `added`, `visited`, `visits` y `name`. Las configuraciones anteriores que no contienen esta preferencia conservan `added`.

En ajustes, **↑/↓**, **Tab** o **Shift+Tab** eligen la opción; **Espacio** o **Enter** cambian su valor. En el orden, **←/→** recorren las opciones en ambos sentidos. Los cambios se guardan automáticamente y se aplican en la siguiente apertura. **Esc** o **F2** vuelven a la vista anterior conservando la búsqueda, el foco y el orden de la sesión. Mientras se escribe en el buscador, las letras —incluida **q**— son texto; **Ctrl+C** sigue permitiendo salir.

Los ajustes pertenecen al usuario, independientemente de `--db`. La prioridad del archivo es:

1. `PROJECTORY_CONFIG_DIR/settings.json`.
2. `XDG_CONFIG_HOME/projectory/settings.json`.
3. La carpeta de configuración del sistema: en macOS, `~/Library/Application Support/projectory/settings.json`; en Linux, `~/.config/projectory/settings.json`.

El archivo contiene la clave booleana `search_on_open` y la clave opcional `default_sort`, por ejemplo `{"search_on_open": true, "default_sort": "name"}`. La escritura reemplaza el archivo completo de forma atómica. Un orden desconocido se rechaza; un error de guardado mantiene los valores anteriores y se muestra en la pantalla de ajustes.

## Navegación directa

```sh
pt ~
pt go ID
pt go /ruta/registrada
pt go "Tienda/copia 2025"
```

`pt ~` vuelve a tu carpeta personal mediante la función del shell, sin abrir SQLite ni registrar una visita. También acepta `pt '~'` y `pt "$HOME"`, siempre como único argumento. `pt` sin argumentos sigue abriendo el selector. Si ya tenías una terminal abierta al instalar esta función, vuelve a activarla con `eval "$("$HOME/.local/bin/projectory" init zsh)"` (o `bash`).

Un nombre de proyecto también funciona si solo tiene una ubicación. Si hay varias coincidencias, el comando pide un ID o una ruta. La visita se registra después de que el shell confirme el cambio de directorio.

Las visitas cuentan entradas mediante `pt`. Un `cd` manual no modifica el contador. El ejecutable sin la función del shell imprime la ruta seleccionada y no puede cambiar el directorio de su proceso padre.

## Edición y eliminación

```sh
pt edit ID
pt edit ID --name "Nuevo nombre" --description "Nuevo contexto"
pt edit ID --label principal --tags cliente,web
pt edit ID --path /nueva/ruta
pt remove ID
pt remove ID --yes
```

Nombre, descripción y tags se comparten entre las ubicaciones del proyecto. Etiqueta y ruta pertenecen a la ubicación. Una cadena vacía limpia la descripción, la etiqueta o los tags.

`remove`, también disponible como `rm`, quita la ubicación del catálogo y conserva sus archivos. Si era la última ubicación, también quita los metadatos del proyecto. Fuera de una terminal interactiva requiere `--yes`.

Un worktree quitado queda excluido del descubrimiento automático. Registrarlo explícitamente con `add` lo restaura. Una carpeta desaparecida permanece en el catálogo como no disponible.

## Actualización y Git

```sh
pt refresh
```

La actualización consulta los repositorios registrados y sus worktrees. No explora todo el disco. Conserva los registros de carpetas desaparecidas para permitir corregir sus rutas.

El detalle consulta la rama y los cambios pendientes de la ubicación seleccionada. Los contadores de commits respecto al upstream usan las referencias disponibles localmente. Projectory no ejecuta `fetch`, `checkout`, `worktree remove` ni `worktree prune`.

La detección de tecnologías lee archivos conocidos en la carpeta registrada. Puede identificar varias tecnologías por ubicación. No instala dependencias ni ejecuta los scripts de los manifiestos.

## Actualizar el programa

```sh
pt update --check
pt update
pt update --yes
```

`--check` consulta la versión estable publicada en GitHub y muestra si es más reciente. Sin `--check`, el comando solicita confirmación antes de descargar e instalar; en un script debes usar `--yes`. No hay consultas automáticas al abrir `pt`.

El actualizador independiente verifica SHA-256, comprueba el ejecutable descargado y lo sustituye mediante un renombrado atómico en la misma carpeta. Conserva los enlaces simbólicos al ejecutable, los permisos, el catálogo y los ajustes. Una descarga o validación fallida conserva el binario anterior. No instala versiones anteriores ni usa `sudo`.

Si Projectory está instalado con Homebrew, usa `brew update && brew upgrade erikwco/tap/projectory`. El actualizador no reemplaza archivos administrados por Brew. Las compilaciones de desarrollo admiten la consulta, pero se actualizan recompilando.

`--prerelease` incluye candidatos de prueba como `1.0.0-rc.1` en la consulta o actualización. Por defecto solo se ofrecen versiones estables. La búsqueda de candidatos considera las 100 publicaciones más recientes. No es necesario autenticarse en GitHub; si su API limita las peticiones, el comando lo indica.

`update` actualiza el programa; `refresh` vuelve a consultar los repositorios que ya están en el catálogo.

## Persistencia y respaldo

```sh
pt info
pt backup /ruta/catalogo-backup.db
pt list --db /ruta/otro-catalogo.db
```

La prioridad de configuración del catálogo es:

1. `--db`: ruta del archivo SQLite.
2. `PROJECTORY_DATA_DIR`: carpeta que contiene `catalog.db`.
3. `XDG_DATA_HOME/projectory/catalog.db`.
4. En macOS, `~/Library/Application Support/projectory/catalog.db`.
5. En Linux, `~/.local/share/projectory/catalog.db`.

SQLite usa WAL, transacciones y una espera de cinco segundos para los bloqueos. Los archivos auxiliares `-wal` y `-shm` pueden aparecer junto al catálogo. `backup` crea una copia consistente y rechaza un destino existente.

`list --json` permite exportar datos para otras herramientas. Esta versión no incluye un importador JSON, sincronización entre equipos ni creación de worktrees.

## Inicialización del shell

```sh
projectory init zsh
projectory init bash --cmd proyectos
```

`init` imprime la definición de la función. Conserva el punto de entrada de la instalación y usa `opt/projectory/bin/projectory` cuando detecta Homebrew, para que una actualización no deje la función apuntando a una versión eliminada. No modifica los archivos de inicio del shell. El nombre predeterminado es `pt`; el nombre personalizado debe ser un identificador válido de shell.

Esta versión proporciona integración con zsh y bash en macOS y Linux. La compatibilidad con otros shells queda fuera de esta versión.
