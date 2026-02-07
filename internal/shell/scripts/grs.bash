# Shell function that lists worktrees via fzf and cd's into the selected one.
# Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
grs() {
    local output
    output=$(grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1)
    if [ -n "$output" ]; then
        if command -v z &> /dev/null; then
            z "$output"
        else
            cd "$output"
        fi
    fi
}
