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
            # Only capture stdout; stderr passes through to the terminal.
            set -l output (command kubegonfig $argv)
            set -l rc $status
            if test $rc -ne 0
                return $rc
            end
            eval $output
        case '*'
            command kubegonfig $argv
    end
end
