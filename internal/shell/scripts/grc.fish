# Shell function that wraps `grove create` to build a new worktree from a
# branch name and immediately cd into it. Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
function grc --description "Grove create - create branch and worktree"
    set -l phrase (string join " " -- $argv)
    set -l output (grove create "$phrase")
    if test $status -eq 0
        if command -q z
            z $output
        else
            cd $output
        end
    else
        echo $output
        return 1
    end
end
