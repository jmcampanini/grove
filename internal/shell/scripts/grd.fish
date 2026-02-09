# Shell function that lists worktrees via fzf and removes the selected one.
# If the current directory was removed, cd's to the main worktree.
# This must be a shell function (not a script) so cd affects the caller's session.
function grd -d "Remove a worktree using fzf"
    set -l output (grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1)
    if test -n "$output"
        set -l main_wt (grove list | head -n1)
        grove remove "$output" $argv
        set -l rc $status
        if not test -d (pwd)
            if test -n "$main_wt"
                cd "$main_wt"
            end
        end
        return $rc
    end
end
