# typed: false
# frozen_string_literal: true

class LetGo < Formula
  desc "A Clojure dialect implemented as a bytecode VM in Go"
  homepage "https://github.com/nooga/let-go"
  license "MIT"
  version "1.4.0"

  on_macos do
    on_intel do
      url "https://github.com/nooga/let-go/releases/download/v1.4.0/let-go_1.4.0_darwin_amd64.tar.gz"
      sha256 "73cdd393d5c6ac27bf2e6af3235918879d23d93a3519449d479e511d3db60fa0"
    end
    on_arm do
      url "https://github.com/nooga/let-go/releases/download/v1.4.0/let-go_1.4.0_darwin_arm64.tar.gz"
      sha256 "a3c71ec0fd6a7df4f2cf6b19fec3d09a6b8709c028c353515fdd3f503d94c826"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/nooga/let-go/releases/download/v1.4.0/let-go_1.4.0_linux_amd64.tar.gz"
      sha256 "657f7ab72f57ce9a3939a77651f6b379648332df67c5c70b8247d5fa7b6b192b"
    end
    on_arm do
      url "https://github.com/nooga/let-go/releases/download/v1.4.0/let-go_1.4.0_linux_arm64.tar.gz"
      sha256 "1ba8f02e074104393daf4fb0482aeb79d575dcdf67efebd458272630c379484b"
    end
  end

  def install
    bin.install "lg"
  end

  test do
    assert_equal "2", shell_output("#{bin}/lg -e '(+ 1 1)'").strip
  end
end
