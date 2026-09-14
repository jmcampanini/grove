package layout

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Remote is the host, owner, and repository name parsed from a git remote URL.
// Owner holds every path segment between the host and the repository, joined
// with "/", so nested groups (host/a/b/repo) keep their structure.
type Remote struct {
	Host  string
	Owner string
	Repo  string
}

// ParseRemoteURL extracts the host, owner, and repository from the URL forms
// git accepts for network remotes: scp-like (git@host:owner/repo.git) and
// scheme URLs (ssh://, git://, http://, https://). A trailing .git is removed.
// Local paths and file:// URLs are rejected because they carry no host, and
// scp-like URLs with a bracketed IPv6 host are rejected as unsupported.
func ParseRemoteURL(raw string) (Remote, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Remote{}, errors.New("remote URL is empty")
	}

	host, path, err := splitRemoteURL(raw)
	if err != nil {
		return Remote{}, err
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")
	if len(segments) < 2 || segments[len(segments)-1] == "" {
		return Remote{}, fmt.Errorf("remote URL %q has no owner and repository path", raw)
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return Remote{}, fmt.Errorf("remote URL %q has an invalid path segment", raw)
		}
	}

	return Remote{
		Host:  host,
		Owner: strings.Join(segments[:len(segments)-1], "/"),
		Repo:  segments[len(segments)-1],
	}, nil
}

func splitRemoteURL(raw string) (host, path string, err error) {
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", "", fmt.Errorf("remote URL %q is not valid: %w", raw, err)
		}
		switch parsed.Scheme {
		case "ssh", "git", "http", "https", "git+ssh", "ssh+git":
		default:
			return "", "", fmt.Errorf("remote URL %q has unsupported scheme %q", raw, parsed.Scheme)
		}
		host = strings.ToLower(parsed.Hostname())
		if host == "" {
			return "", "", fmt.Errorf("remote URL %q has no host", raw)
		}
		return host, parsed.Path, nil
	}

	// scp-like syntax: [user@]host:path. A path with a slash before the
	// first colon is a local path, not a remote.
	colon := strings.Index(raw, ":")
	slash := strings.Index(raw, "/")
	if colon <= 0 || (slash >= 0 && slash < colon) {
		return "", "", fmt.Errorf("remote URL %q has no host", raw)
	}
	host = raw[:colon]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if host == "" {
		return "", "", fmt.Errorf("remote URL %q has no host", raw)
	}
	// A bracketed IPv6 literal contains colons, so the first colon is not
	// the host separator. Grove does not parse this form; use the ssh://
	// URL for the same remote instead.
	if strings.HasPrefix(host, "[") {
		return "", "", fmt.Errorf("remote URL %q uses a bracketed IPv6 host in scp-like form, which is unsupported; use ssh://user@[host]/owner/repo instead", raw)
	}
	return strings.ToLower(host), raw[colon+1:], nil
}
