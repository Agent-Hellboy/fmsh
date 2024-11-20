# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Fmsh < Formula
  desc "File Management Shell"
  homepage "https://github.com/Agent-Hellboy/fmsh"
  version "0.1.7"
  license "GPL-3.0"

  on_macos do
    on_intel do
      url "https://github.com/Agent-Hellboy/fmsh/releases/download/v0.1.7/fmsh_0.1.7_darwin_amd64.tar.gz"
      sha256 "6bacf71da84ebe2a7e95dd1a8ce2bea176d38201f3008eba0a6f314fc6149d3a"

      def install
        bin.install "fmsh"
      end
    end
    on_arm do
      url "https://github.com/Agent-Hellboy/fmsh/releases/download/v0.1.7/fmsh_0.1.7_darwin_arm64.tar.gz"
      sha256 "3c43c3e115b4e3312e73f3c20d6390e1d1650b046ddd3eed9d8212410ecf6775"

      def install
        bin.install "fmsh"
      end
    end
  end

  on_linux do
    on_intel do
      if Hardware::CPU.is_64_bit?
        url "https://github.com/Agent-Hellboy/fmsh/releases/download/v0.1.7/fmsh_0.1.7_linux_amd64.tar.gz"
        sha256 "3a4654ab72dd3a20e4812b9e5470162a57b2197b144ff718c57e3fd4d24ce9cf"

        def install
          bin.install "fmsh"
        end
      end
    end
    on_arm do
      if Hardware::CPU.is_64_bit?
        url "https://github.com/Agent-Hellboy/fmsh/releases/download/v0.1.7/fmsh_0.1.7_linux_arm64.tar.gz"
        sha256 "8e165a0f6fde841c707a72be1169e26eaff58114e3158674d70496a0af423596"

        def install
          bin.install "fmsh"
        end
      end
    end
  end
end
