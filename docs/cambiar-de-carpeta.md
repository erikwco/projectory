# Cambiar la carpeta de trabajo

Esta guía traslada el código mediante una copia verificada. Conserva la carpeta original hasta que compruebes que el destino compila y funciona.

## Preparar la instalación para probar

Desde la carpeta actual del proyecto, instala el ejecutable:

```sh
make install
```

Activa `pt` usando el binario instalado:

```sh
eval "$("$HOME/.local/bin/projectory" init zsh)"
whence -w pt
pt --version
```

`whence` debe indicar `pt: function`. `pt` reside como función en esa sesión del shell. El archivo instalado sigue llamándose `projectory`.

Para activar la función en nuevas terminales, guarda esta línea una sola vez en `~/.zshrc`:

```sh
eval "$("$HOME/.local/bin/projectory" init zsh)"
```

Usar el binario instalado evita que la función dependa de la carpeta del código fuente. Si elegiste otro `PREFIX` al instalar, usa la ruta correspondiente.

## Conservar el contenido del destino

Después de probar la herramienta, define las rutas del traslado:

```sh
projectory_source="$HOME/dev/projectory"
projectory_destination="$HOME/projects/projectory"
```

Sustituye esos ejemplos por las rutas de tu equipo. Comprueba ambas carpetas antes de copiar:

```sh
ls -la "$projectory_source"
ls -la "$projectory_destination"
```

Si el destino ya contiene archivos, conserva una copia de todo su contenido, incluidos los archivos ocultos:

```sh
projectory_destination_backup="$(mktemp -d "$HOME/projectory-before-move.XXXXXX")"
cp -Rp "$projectory_destination/." "$projectory_destination_backup/"
printf 'Respaldo del destino: %s\n' "$projectory_destination_backup"
```

## Copiar y verificar el proyecto

Revisa primero los cambios que haría la copia:

```sh
rsync -avn --exclude '/bin/' "$projectory_source/" "$projectory_destination/"
```

Las barras finales copian el contenido del proyecto directamente al destino. El comando no crea otra carpeta `projectory` dentro del destino. El nuevo `go.mod` y `go.sum` deben viajar con el código porque contienen sus dependencias.

Aplica la copia después de revisar la salida:

```sh
rsync -av --exclude '/bin/' "$projectory_source/" "$projectory_destination/"
```

La copia conserva el origen y no elimina archivos adicionales del destino. `bin/` se vuelve a generar en la nueva ubicación.

Abre una terminal en el destino y verifica el proyecto:

```sh
cd "$projectory_destination"
make check
make install
eval "$("$HOME/.local/bin/projectory" init zsh)"
```

Abre también esa carpeta como workspace en el editor o la herramienta con la que continúes el desarrollo. Una sesión que conserve la ruta anterior seguiría trabajando en la copia anterior.

## Actualizar la entrada del catálogo

Mover el código no mueve el catálogo SQLite del usuario. Comprueba su ubicación:

```sh
pt info
```

Si registraste la carpeta anterior de Projectory, localiza su ID:

```sh
pt list --plain --query projectory
```

Sustituye `ID_DE_LA_UBICACION_ANTERIOR` por el ID que corresponda y corrige su ruta:

```sh
pt edit ID_DE_LA_UBICACION_ANTERIOR --path "$projectory_destination"
```

Esto conserva las visitas y los metadatos. Si no habías registrado la carpeta anterior, puedes registrar la nueva con `pt add --current` desde el destino.

Prueba `pt list` y `pt dash` desde una carpeta distinta. Cuando el destino esté verificado y las herramientas apunten a él, puedes retirar la copia anterior. Conserva el respaldo del destino hasta revisar que no falta ningún archivo previo.
