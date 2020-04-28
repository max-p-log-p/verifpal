# This file was generated by GoReleaser. DO NOT EDIT.
class Verifpal < Formula
  desc "Cryptographic protocol analysis for students and engineers."
  homepage "https://verifpal.com"
  version "0.13.2"
  bottle :unneeded

  if OS.mac?
    url "https://source.symbolic.software/verifpal/verifpal/uploads/70e0fdd429ab7f3b6e64693078729810/verifpal_0.13.2_macos_amd64.zip"
    sha256 "814d7916a3f1fa7b6fb851b23e81bfd1f8c77c69e679223ccd1a98a6c43689f0"
  elsif OS.linux?
    if Hardware::CPU.intel?
      url "https://source.symbolic.software/verifpal/verifpal/uploads/4e350a2e4580c8d34a8e4d97327f5989/verifpal_0.13.2_linux_amd64.zip"
      sha256 "59dc51a2c44c225e81f3ea363b47315b7fcd4b89b4d2aa9e4d642244dbae3e0a"
    end
    if Hardware::CPU.arm?
      if Hardware::CPU.is_64_bit?
        url "https://source.symbolic.software/verifpal/verifpal/uploads/81502bca5e608e0303b85904aa00233b/verifpal_0.13.2_linux_arm64.zip"
        sha256 "397e47f2cd94e8349e72ef1aff0b110b0bdcb2dae30306e94909830cd81bca23"
      else
        url "https://source.symbolic.software/verifpal/verifpal/uploads/16d0027fe6fe25fe9026d3ce0470c7ff/verifpal_0.13.2_linux_armv6.zip"
        sha256 "c23f0bf83243183aa6b66db149e6c4cda2ffec02dea8a1027426807330cab275"
      end
    end
  end

  def install
    bin.install "verifpal"
  end
end
