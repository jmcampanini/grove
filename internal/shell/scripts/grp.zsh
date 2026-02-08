# Shell function that lists open PRs via fzf, then creates or switches to a
# worktree for the selected PR and cd's into it. Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
#
# FZF column layout: <number>\t<searchable>\t<display>
#   --with-nth 3   → show column 3 (pretty display)
#   {1}            → PR number for pr create and preview
#   cut -f1        → extract PR number after selection
grp() {
    local style="${1:-context}"
    local pr_num
    pr_num=$(grove pr list --fzf | fzf \
        --delimiter '\t' \
        --with-nth 3 \
        --preview "grove pr preview --style $style --fzf {1}" \
        --preview-window 'right:50%:wrap' \
        | cut -f1)
    if [[ -n "$pr_num" ]]; then
        # Validate PR number is numeric (defensive check)
        if ! [[ "$pr_num" =~ ^[0-9]+$ ]]; then
            echo "Invalid PR number: $pr_num" >&2
            return 1
        fi
        local output
        # Don't redirect stderr - let info/error messages display to terminal (matches grs pattern)
        if output=$(grove pr create "$pr_num"); then
            # Prefer zoxide (z) when available (same pattern as grs)
            if command -v z &> /dev/null; then
                z "$output"
            else
                cd "$output"
            fi
        else
            return 1
        fi
    fi
}
