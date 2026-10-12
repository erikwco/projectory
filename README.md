# Projectory

[![Checks](https://github.com/erikwco/projectory/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/erikwco/projectory/actions/workflows/ci.yml)
[![Licencia: MIT](https://img.shields.io/badge/licencia-MIT-blue.svg)](LICENSE)

Projectory guarda tus proyectos, sus copias y sus worktrees de Git en un catálogo local. El comando `pt` abre un selector en la terminal y cambia tu shell a la carpeta elegida.

![Búsqueda del proyecto Tienda, elección del worktree feature/carrito y cambio de carpeta en el shell](docs/media/navigation.gif)

Funciona con zsh y bash en macOS y Linux, tanto en Intel/AMD de 64 bits como en ARM64. La interfaz está en español. Los binarios y sus sumas SHA-256 se publican en [Releases](https://github.com/erikwco/projectory/releases).

## Para qué sirve

- Entrar a un proyecto sin recordar dónde está: escribes parte del nombre, la ruta, la rama o un tag y pulsas Enter.
- Reunir en una sola entrada un repositorio y sus worktrees, aunque estén en carpetas alejadas.
- Agrupar copias o clones independientes del mismo proyecto.
- Consultar la rama, los cambios pendientes y las tecnologías detectadas de cada ubicación.
- Ordenar por registro, última visita, número de visitas o nombre.

## Requisitos

- macOS o Linux con zsh o bash.
- Git, para descubrir worktrees y mostrar el estado del repositorio. Las carpetas sin Git también se pueden registrar.

## Instalación

### Homebrew

```sh
brew install erikwco/tap/projectory
eval "$(projectory init zsh)"
```

En bash, usa `eval "$(projectory init bash)"`. Homebrew descarga el binario correspondiente a tu sistema; no necesitas Go.

### Binario independiente

Descarga de [Releases](https://github.com/erikwco/projectory/releases) el archivo de tu sistema y `checksums.txt`:

| Equipo | Archivo |
| --- | --- |
| macOS con Apple Silicon | `projectory_VERSION_darwin_arm64.tar.gz` |
| macOS con Intel | `projectory_VERSION_darwin_amd64.tar.gz` |
| Linux con Intel/AMD | `projectory_VERSION_linux_amd64.tar.gz` |
| Linux con ARM64 | `projectory_VERSION_linux_arm64.tar.gz` |

`VERSION` es el número sin la `v`, por ejemplo `1.0.0`. Compara la suma de `shasum -a 256 ARCHIVO.tar.gz` con su entrada en `checksums.txt` (en Linux también puedes usar `sha256sum`). Extrae el archivo verificado en una carpeta temporal y, desde esa carpeta, instala:

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 projectory "$HOME/.local/bin/projectory"
eval "$("$HOME/.local/bin/projectory" init zsh)"
```

Conserva los archivos de licencia incluidos en la descarga. Si usas bash, cambia `zsh` por `bash`.

### Activar `pt` en cada terminal

Añade la línea `eval` de tu instalación a `~/.zshrc` o `~/.bashrc`. En macOS, bash suele abrir shells de login que leen `~/.bash_profile`; añade la línea allí si `~/.bashrc` no se carga.

`pt` es una función del shell. Llama a `projectory`, recibe la carpeta elegida y ejecuta `cd` en tu terminal. Conserva la ruta absoluta del ejecutable, así que una instalación manual funciona aunque `~/.local/bin` no esté en tu `PATH`.

```sh
type pt
pt --version
```

`type pt` debe indicar que es una función. Si mueves el binario, vuelve a ejecutar `init` con su nueva ruta. Puedes elegir otro nombre con `init zsh --cmd proyectos`. En Homebrew, la función usa una ruta estable entre versiones.

### Compilar desde el código

Necesitas Go 1.26 o posterior y Make. Con `GOTOOLCHAIN=auto`, Go descarga la versión declarada en `go.mod`.

```sh
git clone https://github.com/erikwco/projectory.git
cd projectory
make install
eval "$("$HOME/.local/bin/projectory" init zsh)"
```

`make install` copia el ejecutable a `~/.local/bin/projectory`. `PREFIX=/ruta` permite cambiar el prefijo. Las compilaciones de desarrollo se actualizan volviendo a compilar.

## Actualizar Projectory

Con Homebrew:

```sh
brew update
brew upgrade erikwco/tap/projectory
```

Con un binario independiente de una versión publicada:

```sh
pt update --check
pt update
```

`--check` consulta si hay una versión nueva. `update` muestra el cambio, pide confirmación y verifica la suma SHA-256 antes de reemplazar el ejecutable. `--yes` permite confirmar desde un script. El catálogo y los ajustes se conservan. Las instalaciones de Homebrew remiten a `brew upgrade`.

Las versiones de prueba solo se consultan con `--prerelease`. Projectory no busca actualizaciones al abrir el selector. Después de actualizar, recarga la línea `eval` en las terminales abiertas para incorporar posibles cambios de la función del shell.

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
| `pt update --check` | Consultar una nueva versión del programa. |
| `pt info` | Mostrar la ruta del catálogo y estadísticas. |
| `pt backup RUTA` | Copiar el catálogo a un archivo nuevo. |
| `projectory init zsh` | Imprimir la función del shell. |

`pt list --plain` y `pt list --json` imprimen el catálogo sin interfaz. La [referencia de comandos](docs/commands.md) describe todas las opciones, teclas y rutas de configuración.

## Datos y privacidad

El catálogo es un archivo SQLite en tu equipo; los ajustes son un archivo JSON aparte. La navegación y la administración del catálogo funcionan sin red. Solo `update` consulta GitHub y, si confirmas, descarga una versión; no envía el catálogo, las rutas de tus proyectos ni los ajustes. `pt info` muestra la ruta del catálogo.

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

Ejecuta los tests de Go con el detector de carreras, las pruebas del ejecutable en pseudoterminales y `go vet`. Las pruebas usan catálogos temporales y necesitan Python 3 y zsh o bash. La CI repite `make check` en Linux y macOS, construye los cuatro archivos descargables y ejecuta cada binario en su plataforma. Con GoReleaser 2.18.3 instalado, `make release-snapshot` reproduce el empaquetado y su verificación sin publicar.

- [CONTRIBUTING.md](CONTRIBUTING.md): cómo proponer cambios.
- [SECURITY.md](SECURITY.md): cómo reportar una vulnerabilidad.
- [Demos](docs/demos.md): cómo se generan los GIF de este README.
- [Decisiones de diseño](docs/design.md) y [guía de interfaz](docs/ui.md).
- [Estado del proyecto](docs/estado-del-proyecto.md): implementación y límites actuales.

## Licencia

Projectory se distribuye bajo la [licencia MIT](LICENSE). Copyright © 2026 Erik Chacón. Las dependencias conservan sus propias licencias.
