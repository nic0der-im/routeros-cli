class Ros < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.3.3"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_arm64.tar.gz"
      sha256 "c88a48f6620d6fd8f2dc72d1042664bd7c9cf88489b593c84a2b6d01c4e8a21d"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_amd64.tar.gz"
      sha256 "2c343c08121b8d567c5ba2f7c1c7211ead475c0cf1d14aac0c3a39f091a9e56c"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_arm64.tar.gz"
      sha256 "776a3ecead840811033f90d4928f1f52abd804773610d0fb12ded95a6eb50270"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_amd64.tar.gz"
      sha256 "7e6627c1423acb55aa06ccb734c31f12177c60a0141b4460e1a1153c3f832c75"
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
    EOS
  end

  test do
    assert_match "ros", shell_output("#{bin}/ros version")
  end
end
