# Shell function that lists worktrees via fzf and cd's into the selected one.
# Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
function grs -d "Switch to a worktree using fzf"
    set -l output (grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1)
    if test -n "$output"
        if command -q z
            z "$output"
        else
            cd "$output"
        end
    end
end
