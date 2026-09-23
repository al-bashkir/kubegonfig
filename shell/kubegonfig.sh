# kubegonfig shell integration for bash and zsh
# Source this file from .bashrc or .zshrc:
#   source /path/to/kubegonfig/shell/kubegonfig.sh
#
# This wraps the kubegonfig binary so that the 'use' subcommand
# is eval'd in the current shell, allowing KUBECONFIG export to take effect.

kubegonfig() {
    local cmd=""
    local arg
    for arg in "$@"; do
        case "$arg" in
            -q|--quiet)
                ;;
            --)
                break
                ;;
            *)
                cmd="$arg"
                break
                ;;
        esac
    done

    case "$cmd" in
        use)
            local has_shell=0
            for arg in "$@"; do
                case "$arg" in
                    -h|--help)
                        command kubegonfig "$@"
                        return $?
                        ;;
                    --shell|--shell=*)
                        has_shell=1
                        ;;
                esac
            done

            local kubegonfig_args=("$@")
            if [[ $has_shell -eq 0 ]]; then
                kubegonfig_args+=(--shell posix)
            fi

            local output
            # Only capture stdout; stderr passes through to the terminal.
            output="$(command kubegonfig "${kubegonfig_args[@]}")"
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
