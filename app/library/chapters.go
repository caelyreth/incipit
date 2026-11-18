package library

import "strings"

// AddChapter makes a chapter divider at the end of a group.
func (l *Library) AddChapter(group GroupID, name string) (ChapterID, error) {
	return l.AddChapterAt(group, name, len(l.order[group]))
}

// AddChapterAt makes a chapter divider in group at index, clamped to its
// entries. It validates before inserting either the chapter or its placement.
func (l *Library) AddChapterAt(group GroupID, name string, index int) (ChapterID, error) {
	l.rev++
	if err := l.fileable(group); err != nil {
		return 0, err
	}
	id := l.nextChapter
	l.nextChapter++
	l.chapters[id] = &Chapter{ID: id, Group: group, Name: chapterName(name)}
	l.order[group] = insertAt(l.order[group], index, chapterEntry(id))
	return id, nil
}

func chapterName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled Chapter"
	}
	return name
}

// RenameChapter names a chapter; a blank name is "Untitled Chapter".
func (l *Library) RenameChapter(id ChapterID, name string) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	chapter.Name = chapterName(name)
	return nil
}

// MoveChapter puts a chapter divider in a group at index, clamped to its
// entries; moving within the same group reorders it and so reassigns the
// sheets around it.
func (l *Library) MoveChapter(id ChapterID, group GroupID, index int) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	if err := l.fileable(group); err != nil {
		return err
	}
	l.order[chapter.Group] = removeOrderEntry(l.order[chapter.Group], chapterEntry(id))
	chapter.Group = group
	l.order[group] = insertAt(l.order[group], index, chapterEntry(id))
	return nil
}

// DeleteChapter removes a divider. Its sheets stay where they are and
// belong to the chapter before it, or to none.
func (l *Library) DeleteChapter(id ChapterID) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	l.order[chapter.Group] = removeOrderEntry(l.order[chapter.Group], chapterEntry(id))
	delete(l.chapters, id)
	return nil
}
