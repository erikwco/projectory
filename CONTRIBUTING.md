# Contribuir a Projectory

Puedes abrir issues y pull requests en español o en inglés. Las contribuciones incorporadas a Projectory se distribuyen bajo la [licencia MIT](LICENSE).

> **English:** Issues and pull requests are welcome in English or Spanish. Fork the repository, work on a branch, run `make check`, and open a pull request against `main`. Use fictitious data in examples and screenshots. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md), not in public issues.

## Flujo de trabajo

1. Haz un fork del repositorio y clónalo.
2. Crea una rama a partir de `main` para tu cambio.
3. Ejecuta `make check` antes de subir la rama.
4. Abre un pull request contra `main`.

`main` solo acepta cambios mediante pull request. Para fusionar, deben pasar las comprobaciones requeridas de la CI: **Verify (ubuntu-latest)**, **Verify (macos-latest)** y **Secrets**. Secrets ejecuta Gitleaks sobre el historial y rechaza catálogos, credenciales y binarios versionados. También deben quedar resueltas todas las conversaciones de la revisión.

El proyecto tiene un único mantenedor, que es quien fusiona los pull requests. Por eso la protección de `main` no exige un número mínimo de aprobaciones: el mantenedor no puede aprobar sus propios cambios, y sus pull requests pasan por las mismas comprobaciones que los demás.

Mantén separados los cambios de comportamiento y las tareas de formato no relacionadas.

## Preparar el entorno

Necesitas Go 1.26 o posterior, Git, Make, Python 3 y zsh o bash, en macOS o Linux. `go.mod` declara la versión de Go del proyecto.

```sh
git clone https://github.com/TU_USUARIO/projectory.git
cd projectory
make check
```

`make check` ejecuta los tests con el detector de carreras, compila el binario, prueba la CLI y la TUI en pseudoterminales y ejecuta `go vet`.

## Probar un cambio

Compila y abre un catálogo temporal:

```sh
make build
projectory_test_dir="$(mktemp -d)"
export PROJECTORY_DATA_DIR="$projectory_test_dir/data"
export PROJECTORY_CONFIG_DIR="$projectory_test_dir/config"
./bin/projectory add . --name Projectory
./bin/projectory dash
```

Estas variables aíslan la prueba de tu catálogo y tus ajustes habituales. Para probar también el cambio de carpeta:

```sh
eval "$(./bin/projectory init zsh)"
pt
```

Usa `init bash` si tu shell es bash. Al terminar, ejecuta `unset PROJECTORY_DATA_DIR PROJECTORY_CONFIG_DIR`.

## Enviar el cambio

Ejecuta `gofmt` sobre los archivos Go modificados y `make check` antes de abrir el pull request.

Describe el problema, el comportamiento resultante y las pruebas realizadas. Para cambios de teclado o navegación, verifica el comportamiento con **Buscar al abrir** activado y desactivado. Para cambios en la TUI, revisa terminales amplias y estrechas, con temas claros y oscuros. `python3 scripts/preview.py` genera capturas de revisión en `.impeccable/review/`, una carpeta excluida de Git.

Si el cambio altera lo que muestran los GIF del README, regenéralos con `make demos`. Consulta [docs/demos.md](docs/demos.md).

Lee las [decisiones de diseño](docs/design.md) y la [guía de interfaz](docs/ui.md) antes de cambiar la agrupación de proyectos, la persistencia o el flujo de selección.

## Datos ficticios

Usa nombres, rutas y ramas inventados en tests, ejemplos, capturas y GIF, por ejemplo `~/dev/tienda` o `feature/carrito`. No incluyas rutas de tu equipo, nombres de clientes, direcciones de correo, tokens ni catálogos SQLite reales.

## Reportar un problema

Incluye la salida de `pt --version`, el sistema operativo, el shell, el terminal y los pasos para reproducirlo. Indica si **Buscar al abrir** está activado. Sustituye las rutas y los nombres privados antes de adjuntar una captura o una salida JSON. No adjuntes tu catálogo.

Las vulnerabilidades no se reportan en issues públicos. Sigue las instrucciones de [SECURITY.md](SECURITY.md).
