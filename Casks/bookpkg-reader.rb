cask "bookpkg-reader" do
  # Experimental. version and sha256 are placeholders until the first tagged
  # release that ships the reader build job's artifact
  # (bookpkg-reader_v<version>_darwin_arm64.zip); pin both by running
  # scripts/update-cask.sh <tag>, which updates this cask and plainshelf.rb
  # together so the dependency pair stays in sync.
  version "0.10.0"
  sha256 "7640b793d3f5e0d376ad2cce2f2f5e3d7dad58a3eb6e3eeada8ea29364e82049"

  url "https://github.com/voilelab/plainshelf/releases/download/v#{version}/bookpkg-reader_v#{version}_darwin_arm64.zip"
  name "PlainShelf Reader"
  desc "Experimental standalone reader for a single PlainShelf book package"
  homepage "https://github.com/voilelab/plainshelf"

  depends_on arch: :arm64
  depends_on macos: :sonoma

  app "PlainShelfReader.app"

  postflight do
    # The .app is unsigned and unnotarized until code signing lands; clear the
    # quarantine attribute so it opens without a right-click on first launch.
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/PlainShelfReader.app"]
  end

  uninstall quit: "com.voilelab.plainshelf-reader"

  # Only the reader's own data, keyed to com.voilelab.plainshelf-reader. The
  # reader keeps no "Application Support/PlainShelf" store of its own, so these
  # paths must never overlap the plainshelf cask's zap or `brew zap` here would
  # delete the PlainShelf desktop app's library.
  zap trash: [
    "~/Library/WebKit/com.voilelab.plainshelf-reader",
    "~/Library/Caches/com.voilelab.plainshelf-reader",
    "~/Library/HTTPStorages/com.voilelab.plainshelf-reader",
    "~/Library/Preferences/com.voilelab.plainshelf-reader.plist",
    "~/Library/Saved Application State/com.voilelab.plainshelf-reader.savedState",
  ]
end
