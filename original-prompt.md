i want to add a new feature - the ability to create a worktree for a pull request. here’s my first draft idea of how it works:

there are multiple commands
- `pr list`
- `pr create`
- `pr preview`

and a script command for pr create and then `cd` into it. follow the existing pattern.

## `pr list`

it will list the pull requests for the repository. it will use the internal/github package. it will list out the pull requests, one per line. it should present the user with useful information. this should be in a table form. check what we can push out using bubbles and lipgloss. use context7

there should be an `--fzf` flag that lets it print out information in a way thats useful for fzf to be able to parse. see how we do this for the `list` command. the end state is we want to be able to preview the pr using `pr preview` and the output should be sufficient to tell fzf how to preview it.

the default version of this should list only open PRs.

future versions will add extra flags here to list more prs

the goal of the fzf integration is that it should be easy to pick a PR based on the following criteria:
1. pr title, pr number
2. branch name, author information
3. contents of pr body

it should also show which pr is already created locally. this is important information to have and use downstream. think through where this information should be stored.

## `pr preview`

this should be used from inside of `fzf` and used to preview the PR. lean on the `gh` cli and see what it has to offer. the goal is to make it clear what the user is looking at. it should progressively load more information.

so it should immediately load information about the PR that is pulled in from the query, so it should have author name, etc. it should show the important information

Important information here is pr title, pr number, branch name, author information, files changed, contents of pr body

## `pr create`

this should create a local worktree from the pull request. it should take in a number and then do the following:
1. make a local branch of the remote branch. it should use a config field to determine how to name the local branch. the default should be to just use the remote branch name. a possible option should be to use a pattern that lets it be `pr/<pr-num>` as a valid branch name. come up with simple strategies that allow for this
2. make a worktree from that newly created local branch. we should allow the worktree to have a custom prefix rather than the standard one, default it to `pr-` . it should still use the same logic to remove the prefixes. 
3. the output should be a path to the newly created worktree

think through the different customizations and layers here. come up with a clear path from PR number to a worktree path created, and see where customizations will fit in. make sure there are the right config fields available and that there are sane defaults for these. make sure to show me the path from pr number to worktree path and the different logic steps it goes through.

think about what sort of customizations would be necessary and suggest ways that could happen

think through how this works when its already created. as in if i ran this `pr create` on the same PR twice. it should have sane behavior. come up with options and ask me.

## script command

follow the existing pattern. the end goal is that we’re able to show an fzf selection of the PRs and a preview for them, and then when a user selects one, it should create it if it isnt created, and then switch to it. similar to the existing command. follow the same conventions.

