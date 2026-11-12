# Installation notes

The [README](../README.md#install) lists the installation and upgrade commands for Grove's release archives, mise package, and source build.


## Prerequisites

No installation channel installs Git or [GitHub CLI](https://cli.github.com), so install both yourself. Run `gh auth login` before using Grove's GitHub commands.

Source builds also need [mise](https://mise.jdx.dev). On macOS, `brew install git gh mise` installs the required programs.


## Release archives

Choose the archive for your operating system and architecture on the [releases page](https://github.com/jmcampanini/grove/releases). Releases include SHA-256 checksums and GitHub build provenance attestations. Install a newer release by extracting its archive and replacing the installed binary.

If a macOS browser download is quarantined, remove the attribute before running the extracted binary:

```sh
xattr -d com.apple.quarantine grove
```
