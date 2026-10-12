# Projectory

[![Checks](https://github.com/erikwco/projectory/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/erikwco/projectory/actions/workflows/ci.yml)
[![Licencia: MIT](https://img.shields.io/badge/licencia-MIT-blue.svg)](LICENSE)

Projectory guarda tus proyectos, sus copias y sus worktrees de Git en un catálogo local. El comando `pt` abre un selector en la terminal y cambia tu shell a la carpeta elegida.

![Búsqueda del proyecto Tienda, elección del worktree feature/carrito y cambio de carpeta en el shell](docs/media/navigation.gif)

La versión actual es `0.1.0-dev` y sigue en desarrollo. Todavía no hay versiones publicadas ni binarios precompilados: se instala desde el código fuente. Funciona con zsh y bash en macOS y Linux. La interfaz está en español.

## Para qué sirve

- Entrar a un proyecto sin recordar dónde está: escribes parte del nombre, la ruta, la rama o un tag y pulsas Enter.
- Reunir en una sola entrada un repositorio y sus worktrees, aunque estén en carpetas alejadas.
- Agrupar copias o clones independientes del mismo proyecto.
- Consultar la rama, los cambios pendientes y las tecnologías detectadas de cada ubicación.
- Ordenar por registro, última visita, número de visitas o nombre.

## Requisitos

- macOS o Linux con zsh o bash.
- Go 1.26 o posterior para compilar. Con `GOTOOLCHAIN=auto`, Go descarga la versión declarada en `go.mod`.
- Make.
- Git, para descubrir worktrees y mostrar el estado del repositorio. Las carpetas sin Git también se pueden registrar.

## Instalación

Clona el repositorio e instala el ejecutable:

```sh
git clone https://github.com/erikwco/projectory.git
cd projectory
make install
```

`make install` compila y copia el ejecutable a `~/.local/bin/projectory`. Para usar otro prefijo, ejecuta `make install PREFIX=/ruta`.

Activa `pt` en la sesión actual. En zsh:

```sh
eval "$("$HOME/.local/bin/projectory" init zsh)"
```

En bash:

```sh
eval "$("$HOME/.local/bin/projectory" init bash)"
```

`pt` es una función del shell, no un segundo ejecutable. Un programa no puede cambiar la carpeta de la terminal que lo abrió; por eso la función llama a `projectory`, recibe la carpeta elegida y ejecuta `cd` en tu shell. La función guarda la ruta completa del binario, así que funciona aunque `~/.local/bin` no esté en tu `PATH`.

Comprueba que quedó activa:

```sh
type pt
pt --version
```

`type pt` debe indicar que `pt` es una función.

Para tenerla en cada terminal nueva, añade la misma línea `eval` a `~/.zshrc` si usas zsh, o a `~/.bashrc` si usas bash. En macOS, bash suele abrir shells de login que leen `~/.bash_profile`; añade la línea allí si `~/.bashrc` no se carga.

Si mueves el binario o eliges otro `PREFIX`, repite `init` con la nueva ruta. Puedes cambiar el nombre de la función con `init zsh --cmd proyectos`. Al actualizar Projectory, vuelve a ejecutar la línea `eval` en las terminales que ya estaban abiertas.

## Inicio rápido

Entra a la carpeta que quieres guardar y regístrala:

```sh
cd ~/dev/tienda
pt add --current
```

Completa el nombre y, si quieres, la descripción, los tags y una etiqueta para esta ubicación. **Tab** cambia de campo, **Ctrl+S** guarda y **Esc** cancela.

Abre el selector desde cualquier carpeta:

```sh
pt
```

Pulsa **/** y escribe para filtrar. **Enter** o **↑/↓** devuelven el foco al listado. Con el proyecto seleccionado, **Enter** entra en su carpeta. Si el proyecto tiene varias ubicaciones, se abre un selector para elegir una. **Tab** muestra el detalle y **q** sale sin cambiar de carpeta.

`pt ~` vuelve a tu carpeta personal sin abrir el catálogo.

## Worktrees y copias

Si registras la raíz de un repositorio o de un worktree, Projectory añade los demás worktrees vinculados, aunque estén en otras carpetas. Para incorporar worktrees creados después, ejecuta:

```sh
pt refresh
```

Las copias y los clones independientes no se agrupan solos, aunque compartan remoto. Asócialos al proyecto con `--project`:

```sh
pt add ~/dev/tienda --name Tienda --tags cliente,web
pt add ~/archivo/tienda-2025 --project Tienda --label "copia 2025"
```

El nombre, la descripción y los tags pertenecen al proyecto. Cada ubicación conserva su ruta, su etiqueta y sus visitas.

## Administrar el catálogo

```sh
pt dash
```

El dashboard muestra los contadores de visitas y permite agregar (**a**), editar (**e**) y quitar (**d**) ubicaciones. Quitar un registro no borra archivos. **r** actualiza Git, tecnologías y worktrees.

![Registro de una carpeta con el formulario, dashboard y pantalla de ajustes](docs/media/catalog.gif)

Si moviste una carpeta, corrige su ruta desde el formulario de edición o con `pt edit ID --path ~/nueva/ruta`. La ubicación conserva su historial de visitas.

## Ajustes

Pulsa **F2** en el selector o el dashboard, o ejecuta `pt settings`. Los cambios se guardan al momento y se aplican la próxima vez que abras `pt`, `pt list` o `pt dash`.

- **Buscar al abrir**: desactivado por defecto. Sin él, la búsqueda empieza con **/** y las letras funcionan como atajos. Al activarlo, el buscador recibe el foco al abrir y escribir siempre filtra; los atajos pasan a **Ctrl+A/E/D** (agregar, editar, quitar), **Ctrl+R** (actualizar), **Ctrl+O** (orden) y **Ctrl+C** (salir).
- **Orden predeterminado**: `added` (registro, el valor por defecto), `visited` (última visita), `visits` (visitas) o `name` (nombre).

También se pueden cambiar sin abrir la interfaz:

```sh
pt settings --search-on-open=true
pt settings --default-sort=name
```

`pt --sort visits` cambia el orden solo para esa apertura. **o**, o **Ctrl+O** con **Buscar al abrir**, cambia el orden durante la sesión.

## Comandos

| Comando | Uso |
| --- | --- |
| `pt` o `pt list` | Abrir el selector y entrar a una carpeta. |
| `pt dash` | Administrar el catálogo y ver visitas. |
| `pt add` | Registrar una carpeta y sus worktrees. |
| `pt go ID` | Entrar directamente a una ubicación por ID, ruta o nombre. |
| `pt edit ID` | Cambiar metadatos o corregir una ruta. |
| `pt remove ID` | Quitar una ubicación del catálogo sin borrar archivos. |
| `pt refresh` | Actualizar tecnologías, ramas y worktrees. |
| `pt settings` | Cambiar las preferencias de navegación. |
| `pt info` | Mostrar la ruta del catálogo y estadísticas. |
| `pt backup RUTA` | Copiar el catálogo a un archivo nuevo. |
| `projectory init zsh` | Imprimir la función del shell. |

`pt list --plain` y `pt list --json` imprimen el catálogo sin interfaz. La [referencia de comandos](docs/commands.md) describe todas las opciones, teclas y rutas de configuración.

## Datos y privacidad

Projectory no usa red ni envía datos. El catálogo es un archivo SQLite en tu equipo; los ajustes son un archivo JSON aparte. `pt info` muestra la ruta del catálogo.

| Sistema | Catálogo | Ajustes |
| --- | --- | --- |
| macOS | `~/Library/Application Support/projectory/catalog.db` | `~/Library/Application Support/projectory/settings.json` |
| Linux | `~/.local/share/projectory/catalog.db` | `~/.config/projectory/settings.json` |

`--db`, `PROJECTORY_DATA_DIR` y `XDG_DATA_HOME` cambian la ubicación del catálogo; `PROJECTORY_CONFIG_DIR` y `XDG_CONFIG_HOME`, la de los ajustes.

Para consultar Git, Projectory ejecuta comandos locales de solo lectura. No ejecuta `fetch`, `checkout` ni comandos que creen o borren worktrees. Para detectar tecnologías lee los manifiestos de la carpeta registrada; no instala dependencias ni ejecuta scripts.

Para guardar una copia del catálogo:

```sh
pt backup ~/projectory-backup.db
pt list --db ~/projectory-backup.db --json
```

## Desarrollo

```sh
make check
```

Ejecuta los tests de Go con el detector de carreras, las pruebas del ejecutable en pseudoterminales y `go vet`. Las pruebas usan catálogos temporales y necesitan Python 3 y zsh o bash. La CI repite `make check` en Linux y macOS.

- [CONTRIBUTING.md](CONTRIBUTING.md): cómo proponer cambios.
- [SECURITY.md](SECURITY.md): cómo reportar una vulnerabilidad.
- [Demos](docs/demos.md): cómo se generan los GIF de este README.
- [Decisiones de diseño](docs/design.md) y [guía de interfaz](docs/ui.md).
- [Estado del proyecto](docs/estado-del-proyecto.md): implementación y límites actuales.

## Licencia

Projectory se distribuye bajo la [licencia MIT](LICENSE). Copyright © 2026 Erik Chacón. Las dependencias conservan sus propias licencias.
