# Installation notes

The [README](../README.md#install) lists the installation and upgrade commands for Grove's release archives, mise package, Homebrew cask, HEAD formula, and source build.


## Prerequisites

Homebrew installs Git, GitHub CLI, and bash, zsh, and fish completions with Grove. For archive and mise installs, install Git and [GitHub CLI](https://cli.github.com) separately. Run `gh auth login` before using Grove's GitHub commands.

Source builds also need [mise](https://mise.jdx.dev). On macOS, `brew install git gh mise` installs the required programs.


## Release archives

Choose the archive for your operating system and architecture on the [releases page](https://github.com/jmcampanini/grove/releases). Releases include SHA-256 checksums and GitHub build provenance attestations. Install a newer release by extracting its archive and replacing the installed binary.

If a macOS browser download is quarantined, remove the attribute before running the extracted binary:

```sh
xattr -d com.apple.quarantine grove
```


## Switching Homebrew channels

The HEAD formula builds the latest `main` commit, while the cask installs a versioned release. Both install `grove`, so keep only one installed at a time.

Before switching from HEAD to releases, uninstall the formula:

```sh
brew uninstall --formula jmcampanini/grove/grove
brew install --cask jmcampanini/tap/grove
```
