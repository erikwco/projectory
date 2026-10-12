# Demos del README

Los GIF del README se graban con el ejecutable real y datos ficticios:

- [`media/navigation.gif`](media/navigation.gif): busca el proyecto Tienda, elige el worktree `feature/carrito` y muestra el `cd` en el shell.
- [`media/catalog.gif`](media/catalog.gif): registra una carpeta con el formulario, abre el dashboard y la pantalla de ajustes.

## Requisitos

- Python 3.
- zsh.
- Git.
- [VHS](https://github.com/charmbracelet/vhs), con sus dependencias ttyd y ffmpeg.
- Go y Make para compilar el ejecutable.
- Se usa la fuente monoespaciada predeterminada del navegador. Para elegir otra, ejecuta `python3 scripts/demo.py --font-family "NOMBRE"` después de `make build`.

## Generarlos

Desde la raíz del repositorio:

```sh
make demos
```

El objetivo ejecuta `make build` y después `python3 scripts/demo.py`. Al terminar, `docs/media/navigation.gif` y `docs/media/catalog.gif` quedan reemplazados. Revisa los dos GIF antes de incluirlos en un commit.

## Aislamiento

El script crea una carpeta temporal con un nombre aleatorio, del tipo `/tmp/projectory-demo-XXXXXXXX`, y la borra al terminar. Allí genera los proyectos de ejemplo, un repositorio Git con el worktree `feature/carrito`, el catálogo SQLite y los ajustes. Se usa `/tmp` para que las rutas grabadas no incluyan el nombre de usuario del equipo.

El entorno de grabación conserva `HOME` y `PATH`, y aísla el resto mediante variables de entorno:

- `PROJECTORY_DATA_DIR` y `PROJECTORY_CONFIG_DIR` apuntan el catálogo y los ajustes a la carpeta temporal. Tu catálogo no se lee ni se modifica.
- `GIT_CONFIG_GLOBAL=/dev/null` y `GIT_CONFIG_NOSYSTEM=1` evitan tu configuración de Git. Los commits de ejemplo usan un autor ficticio.
- `ZDOTDIR` aísla los archivos de inicio de zsh. La grabación carga explícitamente el `.zshrc` mínimo generado en la carpeta temporal, sin tus alias ni plugins.

Los archivos `.tape` de VHS se generan en la carpeta temporal durante la ejecución y se borran con ella. No se guardan en el repositorio.

## Capturas de revisión

`scripts/preview.py` es otra herramienta. Genera capturas fijas de la TUI en tamaños y temas distintos para revisar cambios de interfaz, y las guarda en `.impeccable/review/`, que está excluida de Git. Sus capturas no se publican en el README.

```sh
make build
python3 scripts/preview.py
python3 scripts/preview.py --forms
```
