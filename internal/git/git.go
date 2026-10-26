package git

import "time"

// WorktreeRefType identifies what kind of ref a worktree has checked out.
type WorktreeRefType int

const (
	// WorktreeRefTypeBranch is a worktree checked out on a local branch.
	WorktreeRefTypeBranch WorktreeRefType = iota
	// WorktreeRefTypeCommit is a worktree with a detached HEAD at a plain commit.
	WorktreeRefTypeCommit
	// WorktreeRefTypeTag is a worktree with a detached HEAD at a tag.
	WorktreeRefTypeTag
)

// WorktreeRef is the branch, tag, or bare commit a worktree has checked out.
// FullBranch and FullTag report whether the ref is of that kind and return it;
// Commit always returns the commit the ref points at.
type WorktreeRef interface {
	Type() WorktreeRefType
	FullBranch() (*LocalBranch, bool)
	FullTag() (*Tag, bool)
	Commit() Commit
}

// Worktree is one entry of git worktree list. Ref is nil for a bare
// worktree, so callers must check it before use.
type Worktree struct {
	AbsolutePath string
	HeadSHA      string
	Locked       bool
	Ref          WorktreeRef
}

// CommitSHA returns HeadSHA when git reported one, otherwise the SHA of
// Ref's commit, or "" when neither is known.
func (w Worktree) CommitSHA() string {
	if w.HeadSHA != "" {
		return w.HeadSHA
	}
	if w.Ref == nil {
		return ""
	}
	return w.Ref.Commit().SHA
}

// Commit is a git commit with the metadata grove displays. It is also the
// WorktreeRef of a detached HEAD that is not at a tag.
type Commit struct {
	CommittedBy string
	CommittedOn time.Time
	SHA         string
	Subject     string
}

var _ WorktreeRef = &Commit{}

// NewCommit builds a Commit from its parts.
func NewCommit(sha, subject string, committedOn time.Time, committedBy string) Commit {
	return Commit{
		CommittedBy: committedBy,
		CommittedOn: committedOn,
		SHA:         sha,
		Subject:     subject,
	}
}

// Type reports WorktreeRefTypeCommit.
func (c Commit) Type() WorktreeRefType { return WorktreeRefTypeCommit }

// FullBranch always reports false: a bare commit is not a branch.
func (c Commit) FullBranch() (*LocalBranch, bool) { return nil, false }

// FullTag always reports false: a bare commit is not a tag.
func (c Commit) FullTag() (*Tag, bool) { return nil, false }

// Commit returns c itself.
func (c Commit) Commit() Commit { return c }

// Tag is a git tag with its tagger metadata. A lightweight tag carries only
// Name and the commit; the other fields are zero.
type Tag struct {
	commit      Commit
	Message     string    // tag message (empty for lightweight tags)
	Name        string    // short tag name (e.g., "v1.0.0")
	TaggedOn    time.Time // tag creation date (zero for lightweight tags)
	TaggerEmail string    // tagger email (empty for lightweight tags)
	TaggerName  string    // tagger name (empty for lightweight tags)
}

var _ WorktreeRef = &Tag{}

// NewTag builds a Tag; pass zero values for message, tagger, and taggedOn
// to describe a lightweight tag.
func NewTag(name string, commit Commit, message, taggerName, taggerEmail string, taggedOn time.Time) Tag {
	return Tag{
		commit:      commit,
		Message:     message,
		Name:        name,
		TaggedOn:    taggedOn,
		TaggerEmail: taggerEmail,
		TaggerName:  taggerName,
	}
}

// Type reports WorktreeRefTypeTag.
func (t Tag) Type() WorktreeRefType { return WorktreeRefTypeTag }

// FullBranch always reports false: a tag is not a branch.
func (t Tag) FullBranch() (*LocalBranch, bool) { return nil, false }

// FullTag returns t.
func (t Tag) FullTag() (*Tag, bool) { return &t, true }

// Commit returns the commit the tag points at, dereferenced for annotated tags.
func (t Tag) Commit() Commit { return t.commit }

// Date returns the most relevant date for the tag, either TaggedOn or Commit().CommittedOn
func (t Tag) Date() time.Time {
	if !t.TaggedOn.IsZero() {
		return t.TaggedOn
	}
	return t.commit.CommittedOn
}

// IsAnnotated returns true if the tag has tagger metadata (annotated tag) vs being lightweight
func (t Tag) IsAnnotated() bool {
	return t.TaggerName != "" || t.TaggerEmail != "" || !t.TaggedOn.IsZero() || t.Message != ""
}

// LocalBranch is a local branch with its upstream tracking state and, when
// checked out, the worktree that holds it.
type LocalBranch struct {
	Ahead                int // Commits ahead of upstream
	Behind               int // Commits behind upstream
	commit               Commit
	IsCheckedOut         bool
	Name                 string // Short branch name (e.g., "main", not "refs/heads/main")
	UpstreamName         string // Short upstream name (e.g., "origin/main"), empty if no upstream
	WorktreeAbsolutePath string // Absolute path to worktree if checked out, empty otherwise
}

var _ WorktreeRef = &LocalBranch{}

// NewLocalBranch builds a LocalBranch from its parts.
func NewLocalBranch(name, upstreamName, worktreeAbsolutePath string, isCheckedOut bool, ahead, behind int, commit Commit) LocalBranch {
	return LocalBranch{
		Ahead:                ahead,
		Behind:               behind,
		IsCheckedOut:         isCheckedOut,
		Name:                 name,
		UpstreamName:         upstreamName,
		WorktreeAbsolutePath: worktreeAbsolutePath,
		commit:               commit,
	}
}

// Type reports WorktreeRefTypeBranch.
func (b LocalBranch) Type() WorktreeRefType { return WorktreeRefTypeBranch }

// FullBranch returns b.
func (b LocalBranch) FullBranch() (*LocalBranch, bool) { return &b, true }

// FullTag always reports false: a branch is not a tag.
func (b LocalBranch) FullTag() (*Tag, bool) { return nil, false }

// Commit returns the branch tip.
func (b LocalBranch) Commit() Commit { return b.commit }

// RemoteBranch is a remote-tracking branch as recorded locally by the last fetch.
type RemoteBranch struct {
	commit     Commit
	Name       string // Short branch name without remote prefix (e.g., "main")
	RemoteName string // Remote name (e.g., "origin")
}

// NewRemoteBranch builds a RemoteBranch from its parts.
func NewRemoteBranch(name, remoteName string, commit Commit) RemoteBranch {
	return RemoteBranch{
		commit:     commit,
		Name:       name,
		RemoteName: remoteName,
	}
}

// Commit returns the remote-tracking tip.
func (b RemoteBranch) Commit() Commit { return b.commit }

// FullName returns the remote-qualified name, such as "origin/main".
func (b RemoteBranch) FullName() string { return b.RemoteName + "/" + b.Name }

// Git is the set of git operations grove needs. Methods documented as
// mutating change repository state; a dry-run implementation logs them
// instead of executing them.
type Git interface {

	// GetCurrentBranch returns the current branch name.
	// Returns "HEAD" if in detached HEAD state.
	GetCurrentBranch() (string, error)

	// GetMainWorktreePath returns the absolute path to the main (primary) worktree.
	// This is the worktree associated with the .git directory, not a linked worktree.
	GetMainWorktreePath() (string, error)

	// GetRemoteURL returns the URL configured for the named remote
	// (remote.<name>.url) as written, without url.*.insteadOf rewriting.
	// Returns an error if the remote does not exist or git fails.
	GetRemoteURL(remoteName string) (string, error)

	// GetWorktreeRoot returns the absolute path to the root of the git tree.
	// If not in a git repository, returns ("", nil).
	// Returns an error only if the git command itself fails (e.g., git not installed).
	GetWorktreeRoot() (string, error)

	// GetCommitSubject returns the first line of the commit message for HEAD.
	GetCommitSubject() (string, error)

	// GetDefaultRemote returns the default remote name.
	// Returns the value of git config remote.pushDefault if set, otherwise returns the fallback parameter.
	GetDefaultRemote(fallback string) (string, error)

	// GetRepoDefaultBranch returns the default branch name from the local
	// remote-tracking HEAD reference (e.g., refs/remotes/origin/HEAD).
	// Returns the branch name (e.g., "main") if the local remote HEAD is configured.
	// Returns ("", nil) if the remote exists but the local remote HEAD is not set.
	// Returns an error if:
	//   - The remote does not exist
	//   - Not in a git repository
	//   - Git command fails (e.g., git not installed)
	// This works in both regular repositories and worktrees.
	GetRepoDefaultBranch(remoteName string) (string, error)

	// GetRemoteDefaultBranch returns the default branch name by querying the
	// remote's live HEAD reference.
	// Returns the branch name (e.g., "main") if the remote HEAD is configured.
	// Returns ("", nil) if the remote exists but the remote HEAD is not set to a branch.
	// Returns an error if the remote does not exist, cannot be contacted, or git fails.
	GetRemoteDefaultBranch(remoteName string) (string, error)

	// ListLocalBranches returns detailed information about all local branches.
	// This includes the branch name, commit SHA, worktree path (if checked out), upstream tracking, and commit subject.
	ListLocalBranches() ([]LocalBranch, error)

	// ListRemoteBranches returns detailed information about all branches on the specified remote.
	// Uses local refs (requires prior fetch to be current).
	ListRemoteBranches(remoteName string) ([]RemoteBranch, error)

	// ListRemotes returns the names of all configured remotes.
	ListRemotes() ([]string, error)

	// ListTags returns all local annotated and lightweight tags with their metadata.
	// Does NOT sync from remote - call SyncTags() first if needed.
	// Returns both annotated and lightweight tags.
	ListTags() ([]Tag, error)

	// BranchExists checks if a branch with the given name already exists.
	BranchExists(branchName string, caseInsensitive bool) (bool, error)

	// RefExists checks if a git ref (branch, tag, remote tracking ref, etc.) resolves to a valid object.
	RefExists(ref string) (bool, error)

	// ListWorktrees returns detailed information about all worktrees in the repository.
	// This includes the path, associated branch (if any), HEAD commit, and various flags.
	ListWorktrees() ([]Worktree, error)

	// CreateWorktreeForNewBranch atomically creates a new branch and worktree.
	// The branch is created starting from the current HEAD and the worktree is created at the given absolute path.
	// Will mutate the current git state.
	// TODO: consolidate with CreateWorktreeForNewBranchFromRef
	CreateWorktreeForNewBranch(newBranchName, worktreeAbsPath string) error

	// CreateWorktreeForNewBranchFromRef atomically creates a new branch and worktree.
	// The branch is created starting from the specified baseRef (or HEAD if empty).
	// The worktree is created at the given absolute path.
	// Will mutate the current git state.
	// TODO: consolidate with CreateWorktreeForNewBranch
	CreateWorktreeForNewBranchFromRef(newBranchName, worktreeAbsPath, baseRef string) error

	// CreateWorktreeForExistingBranch creates a worktree for an existing branch.
	// The worktree is created at the given absolute path and checks out the specified branch.
	// Will mutate the current git state.
	// TODO: consider merging this with the other CreateWorktree- method
	CreateWorktreeForExistingBranch(branchName, worktreeAbsPath string) error

	// FetchRemoteBranch fetches a remote reference and stores it as a local branch.
	// Will mutate the current git state.
	FetchRemoteBranch(remote, remoteRef, localRef string) error

	// FetchRemoteTrackingBranch fetches a single branch from a remote and updates
	// the corresponding remote-tracking ref used by <remote>/<branch>.
	// Will mutate the current git state.
	FetchRemoteTrackingBranch(remoteName, branchName string) error

	// SyncTags fetches and prunes tags from the remote.
	// If remoteName is empty, uses GetDefaultRemote("origin").
	// Will mutate the current git state.
	SyncTags(remoteName string) error

	// FetchRemote fetches from a remote with full sync (prune refs, prune tags, fetch tags).
	// Will mutate the current git state.
	FetchRemote(remoteName string) (output string, err error)

	// DeleteBranch deletes a local branch.
	// If force is true, uses -D (force delete even if not fully merged).
	// Will mutate the current git state.
	DeleteBranch(name string, force bool) error

	// GetStatus returns the human-readable output of git status for the worktree at absPath.
	GetStatus(absPath string) (string, error)

	// IsWorktreeDirty returns true if the worktree at absPath has uncommitted changes
	// (modified, staged, or untracked files).
	IsWorktreeDirty(absPath string) (bool, error)

	// Merge merges the given ref into the current branch using --no-edit.
	// Returns the merge output (e.g., fast-forward info, file change summary).
	// Will mutate the current git state.
	Merge(ref string) (string, error)

	// ResetHard resets the current branch to the given ref, discarding all local changes.
	// Will mutate the current git state.
	ResetHard(ref string) error

	// PruneWorktrees removes stale worktree metadata for worktrees whose directories
	// no longer exist on disk.
	// Will mutate the current git state.
	PruneWorktrees() error

	// RemoveWorktree removes a linked worktree at the given absolute path.
	// If force is true, removes even if there are uncommitted changes.
	// Will mutate the current git state.
	RemoveWorktree(absPath string, force bool) error
}
