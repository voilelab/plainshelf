cask "bookpkg-reader" do
  # Experimental. version and sha256 are pinned by scripts/update-cask.sh <tag>,
  # which updates this cask and plainshelf.rb together so the dependency pair
  # stays in sync; each stable release opens a PR that runs it.
  version "0.14.1"
  sha256 "a68b3fdd6515b7adf398919a74d6ea1634e333faa9a228228d9592f688e6db38"

  url "https://github.com/voilelab/plainshelf/releases/download/v#{version}/bookpkg-reader_v#{version}_darwin_arm64.zip"
  name "PlainShelf Reader"
  desc "Experimental standalone reader for a single PlainShelf book package"
  homepage "https://github.com/voilelab/plainshelf"

  depends_on arch: :arm64
  depends_on macos: :ventura

  app "PlainShelfReader.app"

  postflight_steps do
    # The .app is unsigned and unnotarized until code signing lands; clear the
    # quarantine attribute so it opens without a right-click on first launch.
    run "/usr/bin/xattr",
        args: ["-dr", "com.apple.quarantine", "{{appdir}}/PlainShelfReader.app"]
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
