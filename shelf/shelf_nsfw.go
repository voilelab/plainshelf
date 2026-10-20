package shelf

import "github.com/voilelab/plainshelf/shelf/internal/shelfutil"

func (s *Shelf) IsBookNSFW(folders FolderPath, meta *BookMeta) bool {
	if meta != nil && meta.NSFW {
		return true
	}
	return s.IsNSFWFolder(folders)
}

func (s *Shelf) IsNSFWFolder(folders FolderPath) bool {
	return s.nsfw.IsNSFWFolder(folders)
}

func (s *Shelf) MarksFolders() bool {
	return s.nsfw.HasFolders()
}

func (s *Shelf) NSFWFolderRule(folders FolderPath) (shelfutil.NSFWFolder, bool) {
	return s.nsfw.Match(folders)
}
