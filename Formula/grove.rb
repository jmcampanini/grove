class Grove < Formula
  desc "Git worktree workspace manager"
  homepage "https://github.com/jmcampanini/grove-cli"
  head "https://github.com/jmcampanini/grove-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/jmcampanini/grove-cli/cmd.Version=#{version}
    ]
    system "go", "build", "-buildvcs=false", *std_go_args(ldflags:)
    generate_completions_from_executable(bin/"grove", "completion")
  end

  test do
    assert_match "grove version HEAD-", shell_output("#{bin}/grove --version")
  end
end
