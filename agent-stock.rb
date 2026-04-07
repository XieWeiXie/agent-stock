# Documentation: https://docs.brew.sh/Formula-Cookbook
#                https://rubydoc.brew.sh/Formula
class AgentStock < Formula
  desc "A command-line tool for AI Agents to access stock market data, including market overview, stock quotes, rankings, and technical indicators."
  homepage "https://github.com/XieWeiXie/agent-stock"
  url "https://github.com/XieWeiXie/agent-stock/archive/v0.1.0.tar.gz"
  sha256 "00d5332a2831cdd8f2399f8afa82076b0ecccfc26a26aa5ad4d5cfecc4816d21"
  license "MIT" # 根据你的 GitHub 仓库信息，我假设是 MIT 许可证。请确认。

  depends_on "go" => :build

  def install
    # The 'std_go_args' helper provides common build flags for Go projects.
    # '-s -w' are linker flags to strip the symbol table and DWARF debugging information,
    # reducing the size of the compiled binary.
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/stock"
    bin.install "stock"
  end

  test do
    # 简单的测试，检查二进制文件是否存在。
    # 如果你的应用有版本命令，可以替换为更具体的测试，例如：
    # assert_match "agent-stock version v0.1.0", shell_output("#{bin}/stock version")
    assert_predicate bin/"stock", :exist?
  end
end
