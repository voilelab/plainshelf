cask "plainshelf" do
  version "0.14.0"
  sha256 "9ea624d24b55d44b6712ccbc0c6ea876788c7cd764f45fe9f35fb68f6ecad984"

  url "https://github.com/voilelab/plainshelf/releases/download/v#{version}/plainshelf-desktop_v#{version}_darwin_arm64.zip"
  name "PlainShelf"
  desc "Local-first personal reading library for plain text books"
  homepage "https://github.com/voilelab/plainshelf"

  depends_on arch: :arm64
  depends_on macos: :ventura
  # Bring in the standalone reader as a cask dependency. The desktop app's
  # "read" action shells out to it (open -n -a PlainShelfReader), so installing
  # plainshelf must also install bookpkg-reader. The reader is deliberately not
  # bundled inside PlainShelf.app; it ships as its own cask.
  depends_on cask: "voilelab/plainshelf/bookpkg-reader"

  app "PlainShelf.app"

  postflight_steps do
    run "/usr/bin/xattr",
        args: ["-dr", "com.apple.quarantine", "{{appdir}}/PlainShelf.app"]
  end

  uninstall quit: "com.voilelab.plainshelf"

  zap trash: [
    "~/Library/Application Support/PlainShelf",
    "~/Library/Caches/PlainShelf",
    "~/Library/WebKit/com.voilelab.plainshelf",
    "~/Library/Caches/com.voilelab.plainshelf",
    "~/Library/HTTPStorages/com.voilelab.plainshelf",
    "~/Library/Preferences/com.voilelab.plainshelf.plist",
    "~/Library/Saved Application State/com.voilelab.plainshelf.savedState",
    "~/Library/WebKit/com.wails.plainshelf",
    "~/Library/Caches/com.wails.plainshelf",
    "~/Library/HTTPStorages/com.wails.plainshelf",
    "~/Library/Preferences/com.wails.plainshelf.plist",
    "~/Library/Saved Application State/com.wails.plainshelf.savedState",
  ]
end
