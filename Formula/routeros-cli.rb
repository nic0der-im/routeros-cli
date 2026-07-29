class Ros < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.3.0"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_arm64.tar.gz"
      sha256 "a926d1fa2ec18b5dfb447fb448d24fed097472028ed4211ccb20fec1351e6d83"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_amd64.tar.gz"
      sha256 "96b81a1c1357bd69d24125b2178de35bce179d225fad61b75d28eacfe95e18c1"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_arm64.tar.gz"
      sha256 "d262b6ad1a6e8313502f4118036b46e0b15c98d67f83ed2ffe0b8a2c45347924"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_amd64.tar.gz"
      sha256 "9bc7be59b01196c5ee377ed6ef144c6465efe4cead708f48c7e3f6ca2c9a571a"
    end
  end

  def install
    bin.install "ros"
    bin.install_symlink "ros" => "routeros-cli"
    generate_completions_from_executable(bin/"ros", "completion")
  end

  def caveats
    <<~EOS
      To get started, add a router:
        echo 'password' | ros device add router-edge --address 192.168.88.1:8728 --username admin --password-stdin

      Agent read-only audit:
        ros -d router-edge --read-only audit -o json

      Completions are installed by this formula. For manual install elsewhere:
        ros completion zsh > "${fpath[1]}/_ros"
    EOS
  end

  test do
    assert_match "ros", shell_output("#{bin}/ros version")
  end
end
