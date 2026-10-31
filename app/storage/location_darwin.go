package storage

// documentsDir is ~/Documents: macOS keeps it there for every user, and
// moves it with the home folder.
func documentsDir() (string, error) {
	return homeDocuments()
}
