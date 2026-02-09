# Shell function that lists worktrees via fzf and removes the selected one.
# If the current directory was removed, cd's to the main worktree.
# This must be a shell function (not a script) so cd affects the caller's session.
grd() {
    local output
    output=$(grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1)
    if [ -n "$output" ]; then
        local main_wt
        main_wt=$(grove list | head -n1)
        grove remove "$output" "$@"
        local rc=$?
        if ! test -d "$(pwd)"; then
            if [ -n "$main_wt" ]; then
                cd "$main_wt"
            fi
        fi
        return $rc
    fi
}
