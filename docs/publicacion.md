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

## Publicar una versión descargable

Las publicaciones usan tags `vMAJOR.MINOR.PATCH`; los candidatos usan `vMAJOR.MINOR.PATCH-rc.N`. El tag debe apuntar a un commit que ya pertenezca a `main`. El workflow `release.yml` vuelve a ejecutar las comprobaciones antes de publicar.

1. Fusiona el PR con todos los checks correctos, sincroniza `main` y comprueba que el árbol esté limpio.
2. Ejecuta `make check` y el ensayo de GoReleaser. Usa la versión de GoReleaser fijada en el workflow:

   ```sh
   make release-snapshot
   ```

3. Crea primero un candidato:

   ```sh
   git tag -a v1.0.0-rc.1 -m 'Projectory 1.0.0-rc.1'
   git push origin v1.0.0-rc.1
   ```

4. Revisa el resultado del workflow **Release** y los archivos de [Releases](https://github.com/erikwco/projectory/releases). Comprueba la instalación y la actualización en carpetas temporales con datos ficticios.
5. Una vez verificado el candidato, crea el tag estable `v1.0.0` sobre el commit aprobado y repite la verificación. Un cambio posterior requiere otro candidato o una nueva versión.

GoReleaser compila con `CGO_ENABLED=0` para Linux/macOS y amd64/arm64. Cada archivo `projectory_VERSION_OS_ARCH.tar.gz` contiene el ejecutable, `LICENSE`, `README.md` y `THIRD_PARTY_NOTICES.md`. Los avisos se generan a partir de las dependencias usadas al compilar. `checksums.txt` contiene las sumas SHA-256.

Activa las releases inmutables antes de publicar. Después de publicadas, sus tags y archivos no se sustituyen ni se borran para corregir errores; se publica otra versión. Consulta y verifica una publicación con:

```sh
gh release view v1.0.0 --repo erikwco/projectory
gh release verify v1.0.0 --repo erikwco/projectory
gh release verify-asset v1.0.0 projectory_1.0.0_darwin_arm64.tar.gz --repo erikwco/projectory
```

Para probar una actualización real desde un binario anterior sin usar tus datos:

```sh
python3 scripts/smoke_update.py --binary /ruta/al/binario-anterior --expected-version 1.0.0
```

Añade `--prerelease` para ensayar un candidato. El script consulta GitHub, usa una instalación y un catálogo temporales y comprueba que la función `pt` abierta siga llamando al binario nuevo. No forma parte de las pruebas sin red de `make check`.

El SHA-256 comprueba la integridad del archivo descargado. La verificación de GitHub comprueba su procedencia mediante la atestación de la release. Las notas de versión deben indicar cambios, plataformas y limitaciones.

## Publicar la actualización de Homebrew

La fórmula vive en [erikwco/homebrew-tap](https://github.com/erikwco/homebrew-tap). Consume los archivos ya publicados y sus hashes. No compila Go ni necesita un token con acceso a los dos repositorios.

El workflow de la tap usa su propio `GITHUB_TOKEN`: valida la release, prueba la fórmula y prepara un PR. Primero ensaya el candidato sin modificar la fórmula estable:

```sh
gh workflow run bump.yml --repo erikwco/homebrew-tap -f version=1.0.0-rc.1 -f dry_run=true
```

Para una versión estable:

```sh
gh workflow run bump.yml --repo erikwco/homebrew-tap -f version=1.0.0 -f dry_run=false
```

Revisa y fusiona el PR generado después de sus pruebas. La tap necesita permitir que GitHub Actions cree pull requests. Los PR creados con `GITHUB_TOKEN` no disparan automáticamente workflows de `pull_request`; la validación se ejecuta dentro del workflow de publicación y su resultado se enlaza en el PR. No configures checks requeridos que nunca vayan a ejecutarse.

Después de fusionar, comprueba:

```sh
brew update
brew install erikwco/tap/projectory
brew test erikwco/tap/projectory
```

En una instalación existente, usa `brew upgrade erikwco/tap/projectory`. Comprueba también `pt` en una terminal que permaneció abierta durante la actualización: debe seguir usando la ruta estable de Homebrew. El catálogo y los ajustes quedan fuera de la instalación.

## Contratos que deben conservarse

- La función `pt` de una terminal abierta puede llamar a un ejecutable más reciente. Mantén compatible `--selection-file` y `_visit` entre actualizaciones compatibles.
- Un binario antiguo rechaza esquemas de catálogo más nuevos. Antes de cambiar el esquema, define la migración y la política de compatibilidad de versiones.
- El comando `update` nunca abre ni migra el catálogo. Comprueba y sustituye solo el ejecutable independiente; las instalaciones de Homebrew se actualizan con Brew.
- No se busca una versión nueva al abrir el selector. Las peticiones de actualización son explícitas.
