class Ros < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.5.0"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_arm64.tar.gz"
      sha256 "46c524a8973b9da7d2a57c62b5a761698824cdc628f0ed05a1d31d2ec870f1b8"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_amd64.tar.gz"
      sha256 "53b9a953a7c575492a3ee8fd59cc78893241ede9dcf8e28c4c6bdfea33fce74f"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_arm64.tar.gz"
      sha256 "903fedd76555ff8d51e2711677e72ff33dcf2eb70fc655d9a96486e7057a1f8a"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_amd64.tar.gz"
      sha256 "54e6a8e741d28eb8f57f443c7826a111be064e37ab97000ff3d2a49f7432e67f"
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
