// Package shell generates the small parent-shell function needed to change cwd.
package shell

import (
	"fmt"
	"regexp"
	"strings"
)

var commandName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Init returns shell code only for installing the function. Selected paths are
// subsequently read as NUL-delimited data, never evaluated as shell code.
func Init(shell, command, binary string) (string, error) {
	if shell != "zsh" && shell != "bash" {
		return "", fmt.Errorf("shell no soportado: %s; usa zsh o bash", shell)
	}
	if !commandName.MatchString(command) {
		return "", fmt.Errorf("el comando debe ser un identificador de shell, por ejemplo pt")
	}
	return fmt.Sprintf(`# Projectory: run 'eval "$(projectory init %s)"' in your shell configuration.
%s() {
  local _pt_file _pt_id _pt_path _pt_db _pt_status _pt_parse
  if [ "$#" -eq 1 ] && { [ "$1" = '~' ] || [ "$1" = "${HOME:-}" ]; }; then
    builtin cd
    return
  fi
  case "${1:-list}" in
    list|dash|go|-*)
      _pt_file="$(command mktemp "${TMPDIR:-/tmp}/projectory.XXXXXXXX")" || return
      %s "$@" --selection-file "$_pt_file"
      _pt_status=$?
      if [ "$_pt_status" -eq 0 ] && [ -s "$_pt_file" ]; then
        {
          IFS= read -r -d '' _pt_id && IFS= read -r -d '' _pt_path && IFS= read -r -d '' _pt_db
        } < "$_pt_file"
        _pt_parse=$?
        if [ "$_pt_parse" -eq 0 ] && [ -n "$_pt_id" ] && [ -n "$_pt_path" ] && [ -n "$_pt_db" ]; then
          if builtin cd -- "$_pt_path"; then
            %s _visit "$_pt_id" --path "$PWD" --db "$_pt_db" || command printf 'Projectory: no se pudo registrar la visita.\n' >&2
          else
            _pt_status=1
          fi
        else
          command printf 'Projectory: selección inválida.\n' >&2
          _pt_status=1
        fi
      fi
      command rm -f -- "$_pt_file"
      return "$_pt_status"
      ;;
    *) %s "$@" ;;
  esac
}
`, shell, command, quote(binary), quote(binary), quote(binary)), nil
}
