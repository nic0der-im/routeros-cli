class Ros < Formula
  desc "CLI tool for managing MikroTik RouterOS routers with structured JSON output"
  homepage "https://github.com/nic0der-im/routeros-cli"
  license "MIT"
  version "0.2.0"

  on_macos do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_arm64.tar.gz"
      sha256 "c024be3bd789606b47d7b3319729848e73a55c15791e4fdb20a1bf47b10e07eb"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_darwin_amd64.tar.gz"
      sha256 "41e4384f44a3d05f7bda23644e3306385f7fa15c129d046b99778c66bee05d2c"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_arm64.tar.gz"
      sha256 "3b25b9e73170a08c621dbaa738a9c8abc718d83256b770a6e2e372e2da1de2b4"
    end
    on_intel do
      url "https://github.com/nic0der-im/routeros-cli/releases/download/v#{version}/ros_#{version}_linux_amd64.tar.gz"
      sha256 "db6f702e862c2a883d4b35ab961134a0da9552d9887a9b758d582fce46fbd794"
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
