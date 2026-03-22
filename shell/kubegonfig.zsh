# kubegonfig shell integration for zsh
# Source this file from .zshrc:
#   source /path/to/kubegonfig/shell/kubegonfig.zsh
#
# This wraps the kubegonfig binary so that 'use' and 'env' subcommands
# are eval'd in the current shell, allowing KUBECONFIG export to take effect.

kubegonfig() {
    local cmd="${1:-}"

    case "$cmd" in
        use|env)
            local output
            # Only capture stdout; stderr passes through to the terminal.
            output="$(command kubegonfig "$@")"
            local rc=$?
            if [[ $rc -ne 0 ]]; then
                return $rc
            fi
            eval "$output"
            ;;
        *)
            command kubegonfig "$@"
            ;;
    esac
}
