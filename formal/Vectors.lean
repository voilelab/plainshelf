import Formal

def vectorFiles : List (String × String) :=
  [("../frontend/src/features/sources/utils/pageOrder.vectors.json", PageOrder.vectorsJson),
   ("../frontend/src/utils/folderOrder.vectors.json", FolderOrder.vectorsJson),
   ("../frontend/src/features/reader/utils/readingPosition.vectors.json",
     ReaderProgress.vectorsJson),
   ("../frontend/src/features/sources/composables/sourceOffsets.vectors.json",
     SourceOffsets.vectorsJson)]

def main : IO Unit := do
  for (path, json) in vectorFiles do
    IO.FS.writeFile path json
