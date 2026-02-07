# Shell function that wraps `grove create` to build a new worktree from a
# branch name and immediately cd into it. Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
grc() {
    local output
    output=$(grove create "$*")
    if [ $? -eq 0 ]; then
        if command -v z &> /dev/null; then
            z "$output"
        else
            cd "$output"
        fi
    else
        echo "$output"
        return 1
    fi
}
