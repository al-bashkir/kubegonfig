# kubegonfig shell integration for fish
# Source this file from config.fish:
#   source /path/to/kubegonfig/shell/kubegonfig.fish
#
# This wraps the kubegonfig binary so that 'use' and 'env' subcommands
# are eval'd in the current shell, allowing KUBECONFIG export to take effect.

function kubegonfig --wraps=kubegonfig
    set -l cmd $argv[1]

    switch "$cmd"
        case use env
            set -l has_shell_flag 0
            for arg in $argv
                switch "$arg"
                    case -h --help
                        command kubegonfig $argv
                        return $status
                    case --shell '--shell=*'
                        set has_shell_flag 1
                end
            end

            # Only capture stdout; stderr passes through to the terminal.
            set -l output
            if test $has_shell_flag -eq 1
                set output (command kubegonfig $argv)
            else
                set output (command kubegonfig $argv --shell fish)
            end
            set -l rc $status
            if test $rc -ne 0
                return $rc
            end
            eval $output
        case '*'
            command kubegonfig $argv
    end
end
