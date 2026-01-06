class Zepctl < Formula
  desc "CLI for administering Zep projects"
  homepage "https://github.com/getzep/zepctl"
  url "https://github.com/getzep/zepctl.git",
      tag:      "v0.0.7",
      revision: "a054a2942944b51b2b1ea6cd26ee66ee87215458"
  license "Apache-2.0"
  head "https://github.com/getzep/zepctl.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/getzep/zepctl/internal/cli.version=#{version}
      -X github.com/getzep/zepctl/internal/cli.commit=#{Utils.git_head}
      -X github.com/getzep/zepctl/internal/cli.date=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/zepctl"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/zepctl version")
  end
end
