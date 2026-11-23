// Package notices tells the user what happened apart from what they did:
// a failure, a recovery, a warning. Any part of the app posts a notice to
// the window's Center, from any goroutine; the window shows each as a
// toast at its foot, in the order they came.
package window

import (
	"sync"
	"time"
)

// NoticeKind is what a notice tells: news, a success, a warning or a failure.
type NoticeKind uint8

const (
	NoticeInfo NoticeKind = iota
	NoticeSuccess
	NoticeWarning
	NoticeError
)

// String is the kind as a toast's Type names it.
func (k NoticeKind) String() string {
	switch k {
	case NoticeSuccess:
		return "success"
	case NoticeWarning:
		return "warning"
	case NoticeError:
		return "error"
	}
	return "info"
}

// shown is how long a notice of the kind shows unless it is sticky: news
// and successes are read at a glance, warnings and failures want time.
func (k NoticeKind) shown() time.Duration {
	if k == NoticeWarning || k == NoticeError {
		return 8 * time.Second
	}
	return 4 * time.Second
}

// Notice is one thing to tell.
type Notice struct {
	// ID names what the notice is about: a notice posted with the ID of
	// one showing takes its place, so a failure that comes again shows
	// once, anew. It is the Title when empty.
	ID   string
	Kind NoticeKind
	// Title says what happened, and Description, if any, why or what
	// follows.
	Title, Description string
	// Sticky keeps the notice until it is closed or dismissed, for a state
	// that lasts, as saving failing.
	Sticky bool
	// Action labels a button that runs OnAction and closes the notice.
	Action   string
	OnAction func()
}

// id is the notice's ID, its Title when it has none.
func (n Notice) id() string {
	if n.ID != "" {
		return n.ID
	}
	return n.Title
}

// Center holds the notices posted since the window last showed them. Its
// zero value is ready; Post, Report and Dismiss are safe from any
// goroutine.
type NoticeCenter struct {
	mu      sync.Mutex
	pending []noticeChange
	wake    func()
}

// change is a notice to show, or with only dismiss, the ID of one to
// close.
type noticeChange struct {
	notice  Notice
	dismiss string
}

// SetWake has the center call wake when a notice is posted or dismissed,
// for the window to show it: wake is called from the goroutine that
// posted, as mygo.Window.Invalidate may be.
func (n *NoticeCenter) SetWake(wake func()) {
	n.mu.Lock()
	n.wake = wake
	n.mu.Unlock()
}

// Post shows a notice.
func (n *NoticeCenter) Post(notice Notice) {
	n.push(noticeChange{notice: notice})
}

// Report tells that title failed, as err says: an error under its title,
// which a later failure of the same takes the place of. A nil err reports
// nothing.
func (n *NoticeCenter) Report(title string, err error) {
	if err == nil {
		return
	}
	n.Post(Notice{Kind: NoticeError, Title: title, Description: describeNotice(err)})
}

// Dismiss closes the notice with id, if it shows.
func (n *NoticeCenter) Dismiss(id string) {
	if id == "" {
		return
	}
	n.push(noticeChange{dismiss: id})
}

func (n *NoticeCenter) push(next noticeChange) {
	n.mu.Lock()
	n.pending = append(n.pending, next)
	wake := n.wake
	n.mu.Unlock()
	if wake != nil {
		wake()
	}
}

// take returns the changes posted since the last take.
func (n *NoticeCenter) take() []noticeChange {
	n.mu.Lock()
	defer n.mu.Unlock()
	pending := n.pending
	n.pending = nil
	return pending
}
