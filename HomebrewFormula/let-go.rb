# typed: false
# frozen_string_literal: true

class LetGo < Formula
  desc "A Clojure dialect implemented as a bytecode VM in Go"
  homepage "https://github.com/nooga/let-go"
  license "MIT"
  version "1.7.4"

  on_macos do
    on_intel do
      url "https://github.com/nooga/let-go/releases/download/v1.7.4/let-go_1.7.4_darwin_amd64.tar.gz"
      sha256 "c1dad618dc619e12053bb5176fc6373c1e7e836bcf10a43773e42787d9c05076"
    end
    on_arm do
      url "https://github.com/nooga/let-go/releases/download/v1.7.4/let-go_1.7.4_darwin_arm64.tar.gz"
      sha256 "297ada0f408744b24e3dbdb470239e3102df3c501136f062cac521d713cafe02"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/nooga/let-go/releases/download/v1.7.4/let-go_1.7.4_linux_amd64.tar.gz"
      sha256 "7053b643f74ad02f0a0676fc2897d3138b13d634f0de7ddf51d6ec7767dae389"
    end
    on_arm do
      url "https://github.com/nooga/let-go/releases/download/v1.7.4/let-go_1.7.4_linux_arm64.tar.gz"
      sha256 "873284d627d4499c9b4524fef006ee5ea640608858b1559245d91efc8f67fc0f"
    end
  end

  def install
    bin.install "lg"
  end

  test do
    assert_equal "2", shell_output("#{bin}/lg -e '(+ 1 1)'").strip
  end
end
