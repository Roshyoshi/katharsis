# UI lane: story hooks to wire (from the story-map pass, 10-15)

Held commit `xpyomooq` (story: cut the RTFM story tail) lands with the UI
lane: rebase it into the lane, then remove `storyTail` (screens/rtfm), its
tests and goldens, and GRUB's `/proc/story` (D53). Keep `StoryLog` exported
and `BeatTitle` (beat pages still use it).

- Scenes, once each, recorded Seen: `story.first-act2` (first route past
  Boss I), `story.first-act3` (past Boss II), `story.first-skip` (first act
  III skip), `story.first-locked` (first capability-locked exit; never on a
  prestige lock).
- Biome fall: try `biome.intro.<node id>` then `biome.intro.<topic>`;
  Kernel Space is node `kernel` (`biome.intro.kernel`). Its route blurb
  needs `names.go` (engine lane): "Any topic, every privilege, and no MMU to
  catch you."
- Boss cards: `boss.rule.<key>` on the scene card and the boss door card
  (TestBossRules keeps it within 60 cols); `boss.oneshot` on one-shot
  bosses; `boss.retry` when a boss fail leaves cycles and deals a new boss
  door (instead of door.* on boss doors).
- Merge window between acts: `fountain.drink` (fountain gave cycles),
  `fountain.full` (gave none), `fountain.dry` (P5+); `item.fsck` after f.
- dmesg strip: `door.timeout`, `door.gave-up` (only when the boot goes on);
  `door.retreat`; `door.retreat.locked` with {time} = the lock time, only
  when no swap is left (kexec included); `verdict.WA.ecc` instead of
  `verdict.WA` when ECC applies.
- Map hub: `map.suspend`, `map.resume {boot}`, `map.abandon {boot}` in the
  n confirm; route `route.locked {module}`, `route.locked.prestige {ring}`.
- ctrl+c in a door: `quit.warn` (story half, unchanged text) plus the rules
  half: "no quitting inside a door: solve · R retreat −5 · :q! give up −10 ·
  closing the terminal does not stop the clock".
- Mechanic wording: "burned 2 cycles, 22 left"; boss card "a fail or :q!
  burns 20 cycles and deals a new boss door (22 left)"; regression card
  "beat you 3 days ago" (not "crashed you").
- Automatic (no wiring): reveal.prestige.5/.7, cleared.5-8,
  grub.subtitle.prestige.5-8 via Ring().
- Engine lane: "+1 life"/"out of lives" strings in flavor/names.go and
  kernel.go (meta.md §(i)); division names for P5-P8 (TestPrestige).
