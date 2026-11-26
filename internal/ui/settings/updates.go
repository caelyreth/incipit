package settings

import (
	"math"
	"strconv"
	"strings"
	"time"

	appupdater "github.com/caelyreth/neverland/internal/app/updater"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	updatesVersionKey     = "updates-version"
	updatesPreferencesKey = "updates-preferences"
)

type updatesNotesState struct {
	open    bool
	version string
}

func (s *State) updatesSections(add func(key, title, description string, match func() bool), only, query string) {
	categoryWords := locales.CategoryUpdates() + " " + locales.CategoryUpdatesDescription()
	for _, item := range []struct {
		key, title, desc, extra string
	}{
		{
			key:   updatesVersionKey,
			title: locales.UpdatesVersionSection(),
			desc:  locales.UpdatesVersionDescription(),
			extra: categoryWords + " " + locales.UpdatesButtonCheck() + " " + locales.UpdatesReleaseNotes() + " " + locales.UpdatesButtonDownload() + " " + locales.UpdatesButtonRestart(),
		},
		{
			key:   updatesPreferencesKey,
			title: locales.UpdatesPreferencesSection(),
			desc:  locales.UpdatesPreferencesDescription(),
			extra: categoryWords + " " + locales.UpdatesAutomaticChecks() + " " + locales.UpdatesAutomaticDownloads() + " " + locales.UpdatesChannel() + " " + locales.UpdatesChannelStable() + " " + locales.UpdatesChannelBeta() + " " + locales.UpdatesKeywords(),
		},
	} {
		if only != "" && item.key != only {
			continue
		}
		item := item
		add(item.key, item.title, item.desc, func() bool {
			return query == "" || containsFold(item.title+" "+item.desc+" "+item.extra, query)
		})
	}
}

func (s *State) updatesSection(c *ui.Context, semantic theme.Theme, key, _ string) {
	switch key {
	case updatesPreferencesKey:
		s.updatesPreferencesCard(c, semantic)
	default:
		s.updatesVersionCard(c, semantic)
	}
}

func (s *State) updatesVersionCard(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	animate := s.motion(c)
	snap := s.Updates
	if snap.CurrentVersion == "" {
		snap.CurrentVersion = mygo.App.Version()
	}

	ui.Column(c).Key("updates-version-section").WidthPercent(100).Children(func() {
		components.SectionHeader{
			Label:       locales.UpdatesVersionSection(),
			Description: locales.UpdatesVersionDescription(),
		}.View(c, semantic)

		hasNotes := snap.Available != nil && strings.TrimSpace(snap.Available.Notes) != ""
		components.SettingGroup{
			Key:   "settings-group-updates-version",
			Label: locales.UpdatesVersionSection(),
			Children: func() {
				s.updateStatusRow(c, semantic, snap, !hasNotes, animate, scale)
				if hasNotes {
					s.updateNotesRow(c, semantic, snap.Available, animate, scale)
				}
			},
		}.View(c, semantic)
	})
}

func (s *State) updateStatusRow(
	c *ui.Context,
	semantic theme.Theme,
	snap appupdater.Snapshot,
	lastRow, animate bool,
	scale float32,
) {
	inset := theme.Space16.DIPs(scale)
	row := ui.Column(c).Key("updates-status-row").WidthPercent(100)
	row.Children(func() {
		ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).
			Gap(theme.Space16.DIPs(scale)).
			Padding(theme.Space16.DIPs(scale), inset, theme.Space16.DIPs(scale), inset).
			Children(func() {
				s.updateStatusBadge(c, semantic, snap, animate, scale)
				ui.Column(c).Grow(1).MinWidth(0).Gap(theme.Space02.DIPs(scale)).Children(func() {
					s.updateHeadline(c, semantic, snap, scale)
					s.updateSubline(c, semantic, snap, scale)
				})
				s.updateActions(c, semantic, snap, animate, scale)
			})

		// Fold the download/install progress bar cleanly beneath the status row.
		showProgress := snap.Status == appupdater.StatusDownloading || snap.Status == appupdater.StatusInstalling
		components.Fold{
			Key:     "updates-progress-fold",
			Open:    showProgress,
			Animate: animate,
			Appear:  true,
		}.View(c, func() {
			s.updateProgressFooter(c, semantic, snap, animate, inset, scale)
		})
	})

	if !lastRow {
		components.HorizontalDivider(c, semantic, components.DefaultDivider).
			Padding(0, inset, 0, inset)
	}
}

func (s *State) updateStatusBadge(
	c *ui.Context,
	semantic theme.Theme,
	snap appupdater.Snapshot,
	animate bool,
	scale float32,
) {
	metrics := theme.DefaultControls.Scaled(scale)
	badgeSize := metrics.Height + theme.Space06.DIPs(scale)
	iconSize := metrics.InlineIcon
	radius := metrics.Radius

	var icon components.IconName
	var fg, bg ui.Color
	switch snap.Status {
	case appupdater.StatusUpToDate, appupdater.StatusReady:
		icon = components.IconCheckCircle
		fg = semantic.Status.Success.Foreground
		bg = semantic.Status.Success.Background
	case appupdater.StatusAvailable, appupdater.StatusDownloading:
		icon = components.IconDownload
		fg = semantic.Accent.Foreground
		bg = semantic.Accent.Tint
	case appupdater.StatusError:
		icon = components.IconError
		fg = semantic.Status.Danger.Foreground
		bg = semantic.Status.Danger.Background
	case appupdater.StatusUnavailable:
		icon = components.IconInfo
		fg = semantic.Text.Muted
		bg = semantic.Surface.Inset
	default:
		icon = components.IconRefresh
		fg = semantic.Accent.Foreground
		bg = semantic.Accent.Tint
	}

	box := ui.Box(c.Key("update-status-badge")).
		Width(badgeSize).Height(badgeSize).Shrink(0).
		Background(bg).Radius(radius).
		Justify(ui.Center).AlignItems(ui.Center)

	if snap.Status == appupdater.StatusDownloading {
		box.Children(func() {
			done := float32(0)
			if snap.Progress > 0 {
				if animate {
					done = components.Follow(box, "ring", float32(snap.Progress), components.MotionNormal)
				} else {
					done = float32(snap.Progress)
				}
			}
			components.ProgressRing(c, "update-ring", iconSize+4, 2, done, semantic.Border.Default, fg, s.updateStatusText(snap))
		})
		return
	}

	box.Children(func() {
		glyph := ui.Icon(c.Key("update-badge-icon"), icon.SVG()).FontSize(iconSize).TextColor(fg)
		if animate && (snap.Status == appupdater.StatusChecking || snap.Status == appupdater.StatusInstalling) {
			spinTarget := ui.Local(box, "spin-target", func() float32 { return 360 })
			if glyph.Settled("spin") {
				*spinTarget += 360
			}
			angle := glyph.AnimateTo("spin", *spinTarget, ui.Animation{
				Duration: 900 * time.Millisecond,
				Ease:     ui.Linear,
			})
			glyph.Rotate(angle)
		}
	})
}

func (s *State) updateHeadline(c *ui.Context, semantic theme.Theme, snap appupdater.Snapshot, _ float32) {
	var title string
	switch {
	case snap.Status == appupdater.StatusAvailable && snap.Available != nil:
		title = locales.UpdatesAvailableTitle(snap.Available.Version)
	case snap.Status == appupdater.StatusReady && snap.Available != nil:
		title = locales.UpdatesReadyTitle(snap.Available.Version)
	case snap.Status == appupdater.StatusUpToDate:
		title = locales.UpdatesUpToDate()
	case snap.Status == appupdater.StatusChecking:
		title = locales.UpdatesChecking()
	case snap.Status == appupdater.StatusError:
		title = locales.UpdatesErrorTitle()
	case snap.CurrentVersion != "":
		title = locales.UpdatesAppVersion(snap.CurrentVersion)
	default:
		title = locales.UpdatesVersionSection()
	}
	components.Label(c, title, theme.TextDefault, semantic.Text.Heading).FontWeight(600)
}

func (s *State) updateSubline(c *ui.Context, semantic theme.Theme, snap appupdater.Snapshot, scale float32) {
	var line string
	ink := semantic.Text.Muted

	switch snap.Status {
	case appupdater.StatusAvailable:
		if snap.ManualOnly() {
			line = locales.UpdatesManualRequired()
			ink = semantic.Status.Warning.Foreground
		} else {
			line = locales.UpdatesAvailablePrompt()
		}
	case appupdater.StatusDownloading:
		pct := 0
		if snap.Progress > 0 {
			pct = int(math.Round(snap.Progress * 100))
		}
		ver := ""
		if snap.Available != nil {
			ver = snap.Available.Version
		}
		line = locales.UpdatesDownloading(ver, strconv.Itoa(pct))
	case appupdater.StatusInstalling:
		line = locales.UpdatesInstalling()
	case appupdater.StatusReady:
		line = locales.UpdatesReadyDescription()
	case appupdater.StatusError:
		if snap.Err != nil {
			line = snap.Err.Error()
		} else {
			line = locales.UpdatesCheckErrorTitle()
		}
		ink = semantic.Status.Danger.Foreground
	case appupdater.StatusUnavailable:
		line = unavailableReason(snap.Availability)
	default:
		versionPart := ""
		if snap.CurrentVersion != "" {
			versionPart = locales.UpdatesCurrentVersion(snap.CurrentVersion)
		}
		checkedPart := lastCheckedLabel(snap.LastCheck)
		if versionPart != "" && checkedPart != "" {
			line = versionPart + " · " + checkedPart
		} else {
			line = versionPart + checkedPart
		}
	}

	ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
		components.Label(c, line, theme.TextSmall, ink)
		if snap.Channel == appupdater.ChannelBeta {
			tagPad := theme.Space04.DIPs(scale)
			components.TagChip(c, locales.UpdatesChannelBeta(), theme.TextXSmall, tagPad, 0, semantic.Accent.Tint, semantic.Accent.Foreground)
		}
	})
}

func (s *State) updateActions(
	c *ui.Context,
	semantic theme.Theme,
	snap appupdater.Snapshot,
	animate bool,
	scale float32,
) {
	ui.Row(c).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Shrink(0).Children(func() {
		switch snap.Status {
		case appupdater.StatusChecking:
			components.Button{
				Key:     "update-cancel-check",
				Label:   locales.UpdatesButtonCancel(),
				Style:   components.SubtleButton,
				Size:    components.CompactButton,
				Animate: animate,
				Action:  callAction(s.UpdateCancel),
			}.View(c, semantic)

		case appupdater.StatusAvailable:
			components.Button{
				Key:     "update-skip",
				Label:   locales.UpdatesButtonSkip(),
				Style:   components.SubtleButton,
				Size:    components.CompactButton,
				Animate: animate,
				Action:  callAction(s.UpdateSkip),
			}.View(c, semantic)
			if snap.ManualOnly() && snap.Available != nil && len(snap.Available.Sources) > 0 {
				url := snap.Available.Sources[0]
				components.Button{
					Key:     "update-manual",
					Label:   locales.UpdatesButtonDownloadManually(),
					Style:   components.PrimaryButton,
					Size:    components.CompactButton,
					Animate: animate,
					Action: func() {
						if s.UpdateOpenURL != nil {
							s.UpdateOpenURL(url)
						}
					},
				}.View(c, semantic)
			} else {
				components.Button{
					Key:     "update-download",
					Label:   locales.UpdatesButtonDownload(),
					Style:   components.PrimaryButton,
					Size:    components.CompactButton,
					Animate: animate,
					Action:  callAction(s.UpdateDownload),
				}.View(c, semantic)
			}

		case appupdater.StatusDownloading:
			components.Button{
				Key:     "update-cancel-download",
				Label:   locales.UpdatesButtonCancel(),
				Style:   components.SubtleButton,
				Size:    components.CompactButton,
				Animate: animate,
				Action:  callAction(s.UpdateCancel),
			}.View(c, semantic)

		case appupdater.StatusInstalling:
			// Signature check & delta patch apply are atomic once download completes.

		case appupdater.StatusReady:
			components.Button{
				Key:     "update-restart",
				Label:   locales.UpdatesButtonRestart(),
				Style:   components.PrimaryButton,
				Size:    components.CompactButton,
				Animate: animate,
				Action:  callAction(s.UpdateRestart),
			}.View(c, semantic)

		case appupdater.StatusError:
			if snap.Available != nil && !snap.ManualOnly() {
				components.Button{
					Key:     "update-skip-err",
					Label:   locales.UpdatesButtonSkip(),
					Style:   components.SubtleButton,
					Size:    components.CompactButton,
					Animate: animate,
					Action:  callAction(s.UpdateSkip),
				}.View(c, semantic)
				components.Button{
					Key:     "update-retry",
					Label:   locales.UpdatesButtonRetry(),
					Style:   components.PrimaryButton,
					Size:    components.CompactButton,
					Animate: animate,
					Action:  callAction(s.UpdateDownload),
				}.View(c, semantic)
			} else {
				components.Button{
					Key:      "update-check-retry",
					Label:    locales.UpdatesButtonCheck(),
					Style:    components.OutlinedButton,
					Size:     components.CompactButton,
					Disabled: !snap.CanCheck,
					Animate:  animate,
					Action:   callAction(s.UpdateCheck),
				}.View(c, semantic)
			}

		default:
			components.Button{
				Key:      "update-check",
				Label:    locales.UpdatesButtonCheck(),
				Style:    components.OutlinedButton,
				Size:     components.CompactButton,
				Disabled: !snap.CanCheck,
				Animate:  animate,
				Action:   callAction(s.UpdateCheck),
			}.View(c, semantic)
		}
	})
}

func (s *State) updateProgressFooter(
	c *ui.Context,
	semantic theme.Theme,
	snap appupdater.Snapshot,
	animate bool,
	inset, scale float32,
) {
	box := ui.Column(c).Key("updates-progress-footer").
		WidthPercent(100).
		Padding(0, inset, theme.Space16.DIPs(scale), inset).
		Gap(theme.Space06.DIPs(scale))
	box.Children(func() {
		val := snap.Progress
		if snap.Status == appupdater.StatusInstalling {
			val = -1
		} else if animate && val >= 0 {
			val = float64(components.Follow(box, "bar", float32(val), components.MotionNormal))
		}
		ui.Progress(c.Key("update-progress-bar"), val).
			WidthPercent(100).
			Height(4).
			TextColor(semantic.Accent.Foreground).
			Background(semantic.Border.Default).
			Label(s.updateStatusText(snap))

		if snap.Total > 0 {
			ui.Row(c).WidthPercent(100).Justify(ui.SpaceBetween).Children(func() {
				components.Label(
					c,
					locales.UpdatesDownloadProgress(components.Bytes(snap.Downloaded), components.Bytes(snap.Total)),
					theme.TextXSmall,
					semantic.Text.Muted,
				)
				if snap.Status == appupdater.StatusDownloading {
					pctBox := ui.Row(c).AlignItems(ui.Center).Gap(1)
					st := ui.Local(pctBox, "pct-roll", func() components.RollingNumber { return components.RollingNumber{} })
					pct := int(math.Round(max(0, min(1, snap.Progress)) * 100))
					pctBox.Children(func() {
						st.View(c, "update-pct-num", pct, theme.TextXSmall.Size(scale), theme.TextXSmall.LineHeight(scale), semantic.Text.Muted, animate)
						components.Label(c, "%", theme.TextXSmall, semantic.Text.Muted)
					})
				}
			})
		}
	})
}

func (s *State) updateNotesRow(
	c *ui.Context,
	semantic theme.Theme,
	rel *appupdater.Release,
	animate bool,
	scale float32,
) {
	if rel == nil {
		return
	}
	metrics := theme.DefaultControls.Scaled(scale)
	inset := theme.Space16.DIPs(scale)
	row := ui.Column(c).Key("updates-notes-row").WidthPercent(100)
	notesState := ui.Local(row, "notes-state", func() updatesNotesState {
		return updatesNotesState{open: true, version: rel.Version}
	})
	if notesState.version != rel.Version {
		notesState.version = rel.Version
		notesState.open = true
	}

	row.Children(func() {
		toggle := ui.Row(c).Key("updates-notes-toggle").
			WidthPercent(100).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), inset).
			OnClick(func() { notesState.open = !notesState.open })
		toggle.Children(func() {
			components.Label(c, locales.UpdatesReleaseNotes(), theme.TextSmall, semantic.Text.Default).FontWeight(500)
			chevron := ui.Icon(c.Key("notes-chevron"), components.IconChevronDown.SVG()).
				FontSize(metrics.InlineIcon).TextColor(semantic.Icon.Muted)
			rot := float32(0)
			if !notesState.open {
				rot = -90
			}
			if animate {
				rot = components.Follow(toggle, "notes-chevron-rot", rot, components.MotionNormal)
			}
			chevron.Rotate(rot)
		})

		components.Fold{
			Key:     "updates-notes-fold",
			Open:    notesState.open,
			Animate: animate,
			Appear:  true,
		}.View(c, func() {
			ui.Column(c).Key("updates-notes-body").
				WidthPercent(100).
				Padding(0, inset, theme.Space16.DIPs(scale), inset).
				Children(func() {
					panel := ui.Column(c).WidthPercent(100).
						Background(semantic.Surface.Inset).
						Radius(metrics.InnerRadius).
						Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), inset).
						Gap(theme.Space06.DIPs(scale))
					panel.Children(func() {
						renderReleaseNotes(c, semantic, rel.Notes, scale)
					})
				})
		})
	})
}

func renderReleaseNotes(c *ui.Context, semantic theme.Theme, raw string, _ float32) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "### "):
			components.Label(c, strings.TrimSpace(strings.TrimPrefix(trimmed, "### ")), theme.TextSmall, semantic.Text.Heading).FontWeight(600)
		case strings.HasPrefix(trimmed, "## "):
			components.Label(c, strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), theme.TextSmall, semantic.Text.Heading).FontWeight(600)
		case strings.HasPrefix(trimmed, "# "):
			components.Label(c, strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")), theme.TextDefault, semantic.Text.Heading).FontWeight(600)
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			body := strings.TrimSpace(trimmed[2:])
			ui.Row(c).WidthPercent(100).MinWidth(0).Gap(6).Children(func() {
				components.Label(c, "•", theme.TextSmall, semantic.Text.Muted).Shrink(0)
				components.Label(c, body, theme.TextSmall, semantic.Text.Default)
			})
		default:
			components.Label(c, trimmed, theme.TextSmall, semantic.Text.Default)
		}
	}
}

func (s *State) updatesPreferencesCard(c *ui.Context, semantic theme.Theme) {
	animate := s.motion(c)
	snap := s.Updates
	ch := snap.Channel
	if ch != appupdater.ChannelBeta {
		ch = appupdater.ChannelStable
	}

	ui.Column(c).Key("updates-preferences-section").WidthPercent(100).Children(func() {
		components.SectionHeader{
			Label:       locales.UpdatesPreferencesSection(),
			Description: locales.UpdatesPreferencesDescription(),
		}.View(c, semantic)

		components.SettingGroup{
			Key:   "settings-group-updates-preferences",
			Label: locales.UpdatesPreferencesSection(),
			Children: func() {
				components.SettingRow{
					Label:       locales.UpdatesAutomaticChecks(),
					Description: locales.UpdatesAutomaticChecksDescription(),
					Control: func() {
						components.Switch{
							Key:      "updates-auto-checks",
							Label:    locales.UpdatesAutomaticChecks(),
							Selected: snap.AutomaticChecks,
							Animate:  animate,
							Action: func(on bool) {
								if s.UpdateSetAutoChecks != nil {
									s.UpdateSetAutoChecks(on)
								}
							},
						}.View(c, semantic)
					},
				}.View(c, semantic)

				components.SettingRow{
					Label:       locales.UpdatesAutomaticDownloads(),
					Description: locales.UpdatesAutomaticDownloadsDescription(),
					Control: func() {
						components.Switch{
							Key:      "updates-auto-downloads",
							Label:    locales.UpdatesAutomaticDownloads(),
							Selected: snap.AutomaticDownloads,
							Animate:  animate,
							Action: func(on bool) {
								if s.UpdateSetAutoDownloads != nil {
									s.UpdateSetAutoDownloads(on)
								}
							},
						}.View(c, semantic)
					},
				}.View(c, semantic)

				components.SettingRow{
					Label:       locales.UpdatesChannel(),
					Description: locales.UpdatesChannelDescription(),
					Last:        true,
					Control: func() {
						components.Segmented{
							Key:     "updates-channel",
							Label:   locales.UpdatesChannel(),
							Animate: animate,
							Options: []components.SegmentOption{
								{
									Key:      appupdater.ChannelStable,
									Label:    locales.UpdatesChannelStable(),
									Selected: ch == appupdater.ChannelStable,
									Action: func() {
										if s.UpdateSetChannel != nil {
											s.UpdateSetChannel(appupdater.ChannelStable)
										}
									},
								},
								{
									Key:      appupdater.ChannelBeta,
									Label:    locales.UpdatesChannelBeta(),
									Selected: ch == appupdater.ChannelBeta,
									Action: func() {
										if s.UpdateSetChannel != nil {
											s.UpdateSetChannel(appupdater.ChannelBeta)
										}
									},
								},
							},
						}.View(c, semantic)
					},
				}.View(c, semantic)
			},
		}.View(c, semantic)
	})
}

func (s *State) updateStatusText(snap appupdater.Snapshot) string {
	switch snap.Status {
	case appupdater.StatusChecking:
		return locales.UpdatesChecking()
	case appupdater.StatusInstalling:
		return locales.UpdatesInstalling()
	case appupdater.StatusDownloading:
		ver := ""
		if snap.Available != nil {
			ver = snap.Available.Version
		}
		pct := 0
		if snap.Progress > 0 {
			pct = int(math.Round(snap.Progress * 100))
		}
		return locales.UpdatesDownloading(ver, strconv.Itoa(pct))
	default:
		return locales.UpdatesVersionSection()
	}
}

func lastCheckedLabel(t time.Time) string {
	if t.IsZero() {
		return locales.UpdatesNeverChecked()
	}
	ago := time.Since(t)
	switch {
	case ago < time.Minute:
		return locales.UpdatesLastCheckedJustNow()
	case ago < time.Hour:
		return locales.UpdatesLastCheckedMinutes(max(1, int(ago.Minutes())))
	case ago < 24*time.Hour:
		return locales.UpdatesLastCheckedHours(max(1, int(ago.Hours())))
	default:
		return locales.UpdatesLastCheckedDays(max(1, int(ago.Hours()/24)))
	}
}

func unavailableReason(av mygo.UpdateAvailability) string {
	switch av {
	case mygo.UpdateUnavailableDev:
		return locales.UpdatesUnavailableDev()
	case mygo.UpdateUnavailableTranslocated:
		return locales.UpdatesUnavailableTranslocated()
	case mygo.UpdateUnavailableReadOnly:
		return locales.UpdatesUnavailableReadOnly()
	case mygo.UpdateUnavailableUnconfigured:
		return locales.UpdatesUnavailableUnconfigured()
	default:
		return locales.UpdatesUnavailablePackage()
	}
}

func callAction(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	return fn
}
