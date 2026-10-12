# Guía de publicación para el mantenedor

Esta guía reúne las comprobaciones antes de cambiar la visibilidad del repositorio en GitHub y antes de preparar una versión. Cambiar la visibilidad es el último paso y lo decide el propietario del repositorio. Subir commits o fusionar pull requests no cambia la visibilidad.

## 1. Sincronizar `main`

Todo lo que se va a publicar debe estar fusionado en `main` y coincidir con el remoto:

```sh
git fetch origin
git switch main
git pull --ff-only
git status --short
git rev-parse main origin/main
```

`git status --short` no debe mostrar cambios y los dos hashes deben coincidir. Revisa también que no queden ramas remotas con trabajo pendiente o con datos que no deban publicarse:

```sh
git branch -r
```

Al hacer público el repositorio se publican todas sus ramas, tags y su historial completo, no solo `main`.

## 2. Revisar secretos y datos personales

- Comprueba que el job **Secrets** de la CI pasa en el último commit de `main`. Ese job ejecuta Gitleaks sobre el historial de todas las ramas descargadas y `scripts/check_public_files.py`, que rechaza catálogos, credenciales, binarios y capturas de revisión versionados.
- Repite la revisión en local. Gitleaks se instala aparte:

  ```sh
  gitleaks git --redact=100 --no-banner --log-opts=--all .
  python3 scripts/check_public_files.py
  ```

- Revisa los autores y committers de los commits. Deben usar una dirección `noreply` de GitHub:

  ```sh
  git log main --format='%an <%ae>%n%cn <%ce>' | sort -u
  ```

- Para los próximos commits y squash merges, activa **Keep my email addresses private** en [Settings → Emails](https://github.com/settings/emails) y comprueba la dirección elegida al fusionar. **Block command line pushes that expose my email** añade una protección para los pushes desde el terminal. Estas preferencias afectan a tu cuenta, no solo a este repositorio.

- Mira los GIF de `docs/media/` fotograma a fotograma o con `make demos` recién ejecutado. Solo deben mostrar los datos ficticios descritos en [demos.md](demos.md).

Si aparece un secreto en el historial, revócalo primero en el servicio que lo emitió. Reescribir el historial no basta: puede haber copias en forks, clones o commits antiguos consultables por SHA en GitHub. Si se reescribió información privada, verifica también esas referencias antes de publicar; la eliminación de copias del servidor puede requerir [contactar a GitHub](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository).

## 3. Comprobar la CI

El workflow [Checks](https://github.com/erikwco/projectory/actions/workflows/ci.yml) debe terminar correctamente en el último commit de `main`, con los jobs **Verify (ubuntu-latest)**, **Verify (macos-latest)** y **Secrets**:

```sh
gh run list --branch main --workflow ci.yml --limit 3
```

Además, desde una clonación nueva, en una carpeta distinta:

```sh
make check
make install
```

Prueba `pt` en zsh y bash. Comprueba que cambia de carpeta y cuenta una sola visita, que el selector de worktrees funciona y que quitar un registro conserva los archivos.

## 4. Proteger `main`

La regla de `main` debe exigir:

- Cambios solo mediante pull request.
- Comprobaciones requeridas: **Verify (ubuntu-latest)**, **Verify (macos-latest)** y **Secrets**.
- Conversaciones resueltas antes de fusionar.
- Cero aprobaciones obligatorias, porque hay un único mantenedor.
- Bloqueo de force push y de borrado de la rama.

Consulta la configuración vigente:

```sh
gh api repos/erikwco/projectory/rulesets
gh api repos/erikwco/projectory/branches/main/protection
```

Uno de los dos comandos debe devolver la regla, según se haya configurado con rulesets o con protección clásica de ramas. En cuentas gratuitas, GitHub solo aplica estas reglas en repositorios públicos. Si es tu caso, aplica la regla inmediatamente después de cambiar la visibilidad y vuelve a comprobarla antes de anunciar el repositorio.

## 5. Revisar la documentación pública

- [LICENSE](../LICENSE) contiene el texto completo de la licencia MIT.
- [SECURITY.md](../SECURITY.md) indica un canal privado para reportar vulnerabilidades. Si ese canal es el reporte privado de GitHub, actívalo en **Settings → Security** después de hacer público el repositorio.
- El [README](../README.md) funciona desde una clonación nueva: instalación, activación de `pt`, inicio rápido y enlaces.
- [CONTRIBUTING.md](../CONTRIBUTING.md) describe la misma protección de `main` que está configurada.

## 6. Cambiar la visibilidad

Cuando los pasos anteriores estén completos, el propietario decide si hace público el repositorio. Puede hacerlo desde **Settings → General → Danger Zone** o con:

```sh
gh repo edit erikwco/projectory --visibility public --accept-visibility-change-consequences
```

Después, comprueba la protección de `main` (paso 4), el canal de seguridad y que los badges del README cargan.

## Preparar una versión descargable

Todavía no hay versiones publicadas. Cuando se prepare la primera, el tag puede ser `v0.1.0`. Verifica la versión incorporada al ejecutable:

```sh
make build VERSION=0.1.0
./bin/projectory --version
```

Publica solo binarios de plataformas verificadas. Incluye la licencia, los avisos que requieran las dependencias y las sumas SHA-256 de los archivos. Las notas de versión deben indicar los sistemas y shells soportados, los cambios y las limitaciones conocidas.
