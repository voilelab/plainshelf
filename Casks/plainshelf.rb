cask "plainshelf" do
  version "0.10.0"
  sha256 "766c30800b72ccfeef286a61b5ad20e9ef705ae2b872e546c6f8ded3ce5ee3e8"

  url "https://github.com/voilelab/plainshelf/releases/download/v#{version}/plainshelf-desktop_v#{version}_darwin_arm64.zip"
  name "PlainShelf"
  desc "Local-first personal reading library for plain text books"
  homepage "https://github.com/voilelab/plainshelf"

  depends_on arch: :arm64
  depends_on macos: :sonoma
  # Bring in the standalone reader as a cask dependency. The desktop app's
  # "read" action shells out to it (open -n -a PlainShelfReader), so installing
  # plainshelf must also install bookpkg-reader. The reader is deliberately not
  # bundled inside PlainShelf.app; it ships as its own cask.
  depends_on cask: "voilelab/plainshelf/bookpkg-reader"

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
