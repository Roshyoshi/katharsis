package app

import (
	tea "charm.land/bubbletea/v2"

	"katharsis/internal/screens/story"
)

// The title screen (story.NewTitle): the fire and the logo, once per
// launch, before GRUB. It goes over GRUB, and over the first-launch
// rating question on GRUB, as the launch's first scene; any key or the
// fire dying down pops it like any every-time scene (sceneDone). A launch
// that resumes a boot shows none: never over a door, and never between a
// boot's doors either, where the boot's own resume scenes play. It plays
// no sound of its own: with no boot, the music's cue is GRUB's
// (audioCue), so GRUB's music starts under the fire.

// titleAtLaunch shows the title when the game sets Deps.Title and the
// launch lands on GRUB. Init calls it, so it shows once per launch.
func (m *Model) titleAtLaunch() tea.Cmd {
	if !m.d.Title || m.state != nil || m.open != nil {
		return nil
	}
	return m.playStory(queued{"", story.NewTitle})
}
