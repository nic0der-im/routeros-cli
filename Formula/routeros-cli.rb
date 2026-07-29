class Ros < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.4.0"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_arm64.tar.gz"
      sha256 "53a1ec04b461a430b19e657b2ae0e8cf784a857ecdb9ea1f46634f9753b76272"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_amd64.tar.gz"
      sha256 "3b87961eb3d0d53059b5c23d727f7c7d7b150065833e4dcb2bc337fe72d3b05f"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_arm64.tar.gz"
      sha256 "7cf09ed8f491bb67101e686d3b884c0a73ab67781ebbdd0dff0d3c358e203b83"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_amd64.tar.gz"
      sha256 "ee6325e1012e4da76d182f6241b3b5a9985e394d17b2d005683bc961f3631e25"
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
        ros -d router-edge --read-only audit
        ros -d router-edge --read-only doctor
    EOS
  end

  test do
    assert_match "ros", shell_output("#{bin}/ros version")
  end
end
