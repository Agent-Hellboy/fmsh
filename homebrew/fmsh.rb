class Fmsh < Formula
    desc "File Management Shell"
    homepage "https://github.com/Agent-Hellboy/fmsh"
    version "v1.0.0" 
    url "https://github.com/Agent-Hellboy/fmsh/releases/download/#{version}/fmsh_#{version}_darwin_amd64.tar.gz"
    sha256 "9b5e9e6b3a0c9e4c9f6c9d4c9f6c9f6c9f6c9f6c9f6c9f6c9f6c9f6c9f6c9f6c"
    license "GPL-3.0"
  
    def install
      bin.install "fmsh"
    end
  
    test do
      # Verify the binary is installed and responds to commands
      assert_match "fmsh", shell_output("#{bin}/fmsh --help")
    end
  end
  