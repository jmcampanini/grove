# Shell function that lists open PRs via fzf, then checks out or switches to a
# worktree for the selected PR and cd's into it. Prefers zoxide (z) over cd.
# This must be a shell function (not a script) so cd affects the caller's session.
#
# FZF column layout: <number>\t<searchable>\t<display>
#   --with-nth 3   → show column 3 (pretty display)
#   {1}            → PR number for pr checkout and preview
#   cut -f1        → extract PR number after selection
grp() {
    local pr_num
    pr_num=$(grove pr list --fzf | fzf \
        --delimiter '\t' \
        --with-nth 3 \
        --preview "grove pr preview --color always --fzf {1}" \
        --preview-window 'right:50%:wrap' \
        | cut -f1)
    if [[ -n "$pr_num" ]]; then
        if ! [[ "$pr_num" =~ ^[0-9]+$ ]]; then
            echo "Invalid PR number: $pr_num" >&2
            return 1
        fi
        local output
        if output=$(grove pr checkout "$pr_num"); then
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
