cask "plainshelf" do
  version "0.8.0"
  sha256 "6d6060fdb55bea52ae74776b613e06da1e2787626441d0791bb0c439e7f24155"

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
