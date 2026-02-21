class Grove < Formula
  desc "Git worktree workspace manager"
  homepage "https://github.com/jmcampanini/grove-cli"
  head "https://github.com/jmcampanini/grove-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/jmcampanini/grove-cli/cmd.Version=HEAD-#{Utils.git_short_head}"
    system "go", "build", *std_go_args(ldflags: ldflags)
    generate_completions_from_executable(bin/"grove", "completion")
  end

  test do
    assert_match "grove version HEAD-", shell_output("#{bin}/grove --version")
  end
end
