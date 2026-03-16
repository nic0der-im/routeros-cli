class RouterosCli < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.1.0"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/routeros-cli_#{version}_darwin_arm64.tar.gz"
      # sha256 "UPDATE_WITH_ACTUAL_SHA256"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/routeros-cli_#{version}_darwin_amd64.tar.gz"
      # sha256 "UPDATE_WITH_ACTUAL_SHA256"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/routeros-cli_#{version}_linux_arm64.tar.gz"
      # sha256 "UPDATE_WITH_ACTUAL_SHA256"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/routeros-cli_#{version}_linux_amd64.tar.gz"
      # sha256 "UPDATE_WITH_ACTUAL_SHA256"
    end
  end

  def install
    bin.install "routeros-cli"
  end

  def caveats
    <<~EOS
      To get started, add a router:
        echo 'password' | routeros-cli device add myrouter --address 192.168.88.1:8728 --username admin --tls=false --password-stdin

      Enable shell completions:
        routeros-cli completion zsh > $(brew --prefix)/share/zsh/site-functions/_routeros-cli
    EOS
  end

  test do
    assert_match "routeros-cli", shell_output("#{bin}/routeros-cli version")
  end
end
