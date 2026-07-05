cask "plainshelf" do
  version "0.7.0"
  sha256 "fe64aa74c18d6bed0ca00f4d6b194d7b35ed03229d493db37dbe16afafcaaec0"

  url "https://github.com/voilelab/plainshelf/releases/download/v#{version}/plainshelf-desktop_v#{version}_darwin_arm64.zip"
  name "PlainShelf"
  desc "Local-first personal reading library for plain text books"
  homepage "https://github.com/voilelab/plainshelf"

  depends_on arch: :arm64
  depends_on macos: :sonoma

  app "PlainShelf.app"

  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/PlainShelf.app"]
  end

  uninstall quit: "com.voilelab.plainshelf"

  zap trash: [
    "~/Library/Application Support/PlainShelf",
    "~/Library/Caches/PlainShelf",
    "~/Library/Preferences/com.voilelab.plainshelf.plist",
  ]
end
