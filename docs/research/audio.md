# Audio for katharsis: licensing, sources, engine

Audio thread, 2026-10-14/15. Covers the two rights layers for Touhou
arrangements, what qualified and what did not, how the music maps to
biomes, the engine and its numbers, and how levels were chosen. The
track data itself is `internal/audio/tracks.toml`; this file is the
evidence behind it.

Labels: **quoted** text is verbatim from the linked page, fetched
2026-10-14 with `curl -A katharsis` (no login, no cookies). *Inferred*
marks my reading, not the source's words. This is not legal advice.

## Summary

- **The music is Touhou chiptune, and it qualifies.** SUZUKI PLAN (Yoji
  Suzuki) publishes 東方BGM on VGS: 175 chiptune arrangements of ZUN's
  music as VGS MML text, with a written permission to use them in Touhou
  fan works within ZUN's guideline (quoted below). katharsis ports
  SUZUKI PLAN's BSD-2 MML compiler and decoder to Go and renders the songs
  as they play. Every song covers a stage, boss, title, ending or staff
  roll of Touhou 1, 4-14, 12.8 and two ZUN music CDs, so each biome gets a
  pool from the matching part of the games.
- **One recording joins them:** SwapXFO's NES (2A03 + N163) "Misty Lake",
  CC BY 4.0 on archive.org.
- **Condition the permission carries:** katharsis must present its music
  as a Touhou Project fan work and stay inside ZUN's guideline (free, not
  passed off as official, no game data). The game now says so on the
  credits page, on `katharsis audio fetch` and in the README. Whether the
  user accepts that label, and where the game is distributed, is the
  user's call (see the end).
- **Excluded:** anything ND or all-rights-reserved, Bandcamp albums (no
  stable download), Mod Archive modules (no game-use license), a netlabel
  release whose two license statements disagree, and anything from
  YouTube or streaming sites.
- **Verification without listening:** the Go port renders the same samples
  as the C decoder for all 176 MML files in tohovgs-cli (first pass plus
  10 s past the loop); all 176 manifest files downloaded and matched their
  pinned SHA-256; loudness, clipping and loop seams were measured.

## 1. ZUN's fan-work guideline (the compositions)

Primary source: 東方Projectの二次創作ガイドライン,
https://touhou-project.news/guideline/ (更新日：2024年5月31日), and the
English page https://touhou-project.news/guidelines_en/ (UPDATE: November
10, 2020). The Japanese page is the newer one.

Fan works are free within common sense:

> 個人でのファン活動としての二次創作は、常識の範囲内で基本的に自由です。
> ただし、以下の条件を守ってください。

> In short, you may simply use your common sense if you're an individual
> or group (as opposed to a legal person) creating Fan Content (stuff you
> make based on Touhou Project) just for fun.

The one required notice:

> 条件
> 東方Projectの二次創作である事を明記してください。

> Your Fan Content should always have: A clear note that your Fan Content
> is a fan work based on Touhou Project.

Prohibitions that bear on music (excerpt):

> 東方Projectの公式コンテンツであると誤解をまねく内容
> スクリーンショット、プレイ動画以外のゲーム素材を使用、公開する行為
> 二次創作物を、その二次創作者に無断で再利用する行為

> Anything that means to mistake your Fan Content as one of the official
> Touhou Project titles. Anything that is extracted from official Touhou
> Project games. [...] Other Touhou Fan Content without the creator's
> permission.

Distribution and money:

> 二次創作物の頒布は、原作が販売されている流通か、許可している流通のみで行えます。
> 具体例 [...] インディーゲームサイト、等
> ブラウザ上で動くゲームアプリ、スマートフォンでのゲームアプリは無料のもののみとします。

> In general, your Fan Content may only be distributed through platforms
> where: you can find official Touhou Project contents [or] they are
> licensed or permitted by ZUN [...] Browser games and mobile games can
> only be released as free-to-play games.

The guideline says earlier guidelines stay in force ("これまでに公開されて
きたガイドラインについても有効です"). Two of them matter here:

- ZUN's 2011-12-20 post on reusing fan works
  (https://kourindou.exblog.jp/17092211/): 「二次創作物の権利（複製や頒布
  など、作品を取り扱うことに関して）はその作品の制作者」が保有しています。ですので、
  二次創作物を再利用したり改変したりする場合は、まず二次創作物の作者に許可を取るよう
  にしてください。また二次創作物を再利用して作られたものも、東方Projectのガイドライ
  ンに沿って活動するようにしてください。 This is why every arrangement needs
  its arranger's license, and why katharsis, built on them, falls under
  the guideline itself.
- Takker's ZUN-reviewed definitions
  (http://takker6.tada-katsu.com/t-081-3.html) count 「耳コピーして作った曲、
  アレンジして作った曲」 as 二次創作物, distinct from 二次使用 (reusing the
  games' own data). The 2004 conditions on using the games' internal BGM
  (http://jiyugiga.sakura.ne.jp/about_zun%27smusic.html, 「東方の二次創作物以
  外のプログラムでの使用は禁止します」) apply to ripped game data. katharsis
  uses none, so they do not apply; *inferred* from the page's own scope
  (「曲等の内部データの使用に関しては」).

**What this asks of katharsis** (*inferred*): it must label itself, or at
least its soundtrack, as a Touhou Project fan work; stay free; never imply
it is official; never ship or fetch ZUN's own game data; and use each
arrangement only with its arranger's permission. A free terminal game
fits. Distribution through GitHub is not on either list of examples; an
indie game site (itch.io) is on the Japanese one. That is the user's
call, since nothing here distributes the game yet.

The notice is `audio.TouhouNotice`, shown at the top of the credits page
(GRUB → Credits, `katharsis audio credits`), printed by `katharsis audio
fetch`, and in the README.

## 2. The arrangers' licenses (the recordings and data)

### 2.1 SUZUKI PLAN, 東方BGM on VGS: qualifies (written permission)

Source: https://github.com/suzukiplan/tohovgs-cli, README, section
"License", at commit `cbb13ada5dd1d9244148a265946310bdd192d4b6` (master,
pushed 2025-08-30). The statement was added on 2022-12-28 in commit
`e6f7e143fb` ("楽曲データの二次利用について"). Quoted:

> - 楽曲（東方 Project）著作権: 楽曲の著作権は上海アリス幻樂団に帰属します
> - データ著作権: 楽曲データの著作権は Yoji Suzuki に帰属する東方 Project 二次創作物です
>   - 本楽曲の東方Projectの二次創作物への利用は [東方Projectの二次創作ガイドライン](https://touhou-project.news/guideline/) で認められている範囲でご自由にお使い頂けます（事前事後の連絡は基本不要ですがご連絡いただければ喜びます）
> - [vgs-bgm-decoder](https://github.com/suzukiplan/vgs-bgm-decoder): このプログラムの著作権は Yoji Suzuki に帰属します（2 箇条 BSD ライセンス）
> - [vgs-mml-compiler](https://github.com/suzukiplan/vgs-mml-compiler): このプログラムの著作権は Yoji Suzuki に帰属します（2 箇条 BSD ライセンス）

Translation: the songs are copyright Team Shanghai Alice; the song data is
copyright Yoji Suzuki, a Touhou Project fan work; it may be used freely in
Touhou Project fan works within what ZUN's guideline allows, with no
notice needed before or after (he is glad to hear from you); the decoder
and compiler are BSD 2-Clause.

The Android app's README points its MML license here: 「楽曲の MML のライセ
ンスは tohovgs-cli のライセンスに基づきます」
(https://github.com/suzukiplan/tohovgs4-android).

**An older, stricter statement exists and does not govern these files.**
https://github.com/suzukiplan/Touhou-VGS-MML-data (README last changed
2016-02-05, repository untouched since 2016-02-21) adds, for its own MML
data, 「1-3 プログラム製品・アプリへの組み込みについて: 利用者は、GitHubアカウント
を有し、本リポジトリを必ずforkするものとします」 (embedding in a program
requires the user to fork that repository). katharsis fetches from
tohovgs-cli, whose 2022 statement is newer and is the one the app points
to, so the fork condition does not apply (*inferred*). Forking
Touhou-VGS-MML-data costs nothing and would satisfy the old wording too;
it is listed as a user call.

**Fetching.** The README asks programs to stay off the app's distribution
server and suggests GitHub instead: 「独自のプログラムから楽曲配信サーバにアク
セスしたい場合は gh-pages からアクセスする方法もあり、それなら私の通信料負担は発生し
ないので助かります」. katharsis fetches
`https://raw.githubusercontent.com/suzukiplan/tohovgs-cli/cbb13ad.../mml/<song>.mml`:
GitHub, like gh-pages, but pinned to a commit so each file's SHA-256 never
changes under it.

**Notices.** The permission asks for none beyond ZUN's label. Credit is
given anyway: "arr. by SUZUKI PLAN" in every toast, and the source block
on the credits page. The BSD 2-Clause license of the ported compiler and
decoder asks binaries to reproduce its text "in the documentation and/or
other materials"; the credits page prints it in full
(`vgs.License`), and the port's source keeps it (`internal/audio/vgs/vgs.go`).

**Not used from the same author:** "Touhou BGM on NSF"
(https://github.com/suzukiplan/tohonsf, FamiStudio projects, 2A03 only)
permits use only in fan games that run on VGS-Zero or a Famicom or its
emulator: 「VGS-Zero または ファミコンの実機 若しくは エミュレータ で動作する
東方Project の二次創作ゲームソフトで自由に使っていただけます」. katharsis is
neither, so it is excluded.

### 2.2 SwapXFO on archive.org: qualifies (CC BY 4.0)

Item https://archive.org/details/mus-misty-lake, "Misty Lake (2A03+N163
Arrange) [Touhou 14]", creator SwapXFO, metadata
`licenseurl: https://creativecommons.org/licenses/by/4.0/`, date
2025-08-13. The account holds 173 items, all with creator SwapXFO and
first-person descriptions ("My second published Touhou cover!"), so the
uploader is the arranger (*inferred*). CC BY 4.0 asks for credit, a link
to the license and a note of changes; katharsis plays the file unmodified
(decoded, gapless-trimmed and resampled, which are not adaptations,
*inferred*) and credits it with the license link and source.

Other SwapXFO Touhou covers qualify on license but were left out:
"Eastern Wind" (Touhou 2) and "DIMENSION OF REVERIE" (Touhou 3) are CC BY
4.0 but only m4a, which the engine does not decode; "Maniacal Princess"
(CC BY-NC 4.0) is Opus only; "Apparitions Stalk the Night" (CC BY-NC
4.0, mp3) is a Nintendo DS-style arrangement, not 8-bit; "Frozen Tea"
names no original. Items titled "Touhou ... Inspired" are originals.

### 2.3 Excluded, with the reason

| What | License evidence | Why excluded |
|---|---|---|
| Kommisar, *Reimu Plays LSDJ* (Game Boy, PC-98 themes), archive.org `Pb005Kommisar-ReimuPlaysLsdj` | item: CC BY-NC-SA 3.0; the label's release page http://www.pxl-bot.com/2011/05/pxl-bot-proudly-bring-you-one-of-our.html carries a site-wide CC BY-NC-ND 3.0 badge | the two statements disagree, one is ND: ambiguous |
| +TEK, *Highly Responsive to FM*, *Story of Modulated Frequency*, *Phantasmagoria of F.M*, *Embodiment of Frequency Modulation* (Bandcamp) | CC BY 4.0 | no stable download URL (Bandcamp's per-session links) |
| Pegmode, *Eastern Retrograde Memories* (LSDj) | CC BY-NC 4.0 | same |
| Zahran Worrell, *Danmaku Matrix Generation*, *Perfect Red*, *Resonance in Blue* (LSDj) | CC BY-NC-SA 4.0 | same; the MP3s on zahranw.com/music_archive carry only "©2026 Zahran Worrell" |
| HertzDevil, *PC-98 Perfect Cherry Blossom*; Saria Lemes; Tangentg; Mille Oculos; Utsuho; Violet Delta; Mastar Music (Bandcamp) | CC BY / BY-SA / BY-NC / BY-NC-SA 4.0 | same |
| HertzDevil *MEGA ZUN*, Grazepoint *TouWHO??* 1-3, Hooded Edge, Nocti *Gensokyo 199X* 1-4 | CC BY-NC-ND 4.0 | ND |
| 65 Touhou albums on Bandcamp tagged chiptune, 8-bit, LSDj, NES and similar (40Nix, Azureflux, Drillimation Systems, BillGP, Stas Gavrik, ...) | "All Rights Reserved" | no license |
| 49 Touhou modules on The Mod Archive (e.g. `sacredlot.s3m`, `groundcherry.it`) | 48 under the "Mod Archive Distribution license"; the site's FAQ: "Can I use a module of modarchive.org in my game? [...] the only way [...] is to contact them and get their permission"; one CC BY-NC (`haru_no_niwa.it`) | no game-use license; tracker formats the engine does not play |
| archive.org Touhou uploads by others (OST rips, "PC 98 Touhou Music", *8BiTouhou 1*, CC0 tags on ZUN's own songs) | none, or a license the uploader cannot grant | provenance |
| OpenGameArt "touhou" results (*Chains of Discord*, *Rin's Theme*, ...) | CC0 and others | original "Touhou-style" pieces, not arrangements |
| Free Music Archive, ccMixter, Jamendo, Wikimedia Commons, Newgrounds, Battle of the Bits | FMA downloads redirect to a login; ccMixter 0 Touhou hits; Newgrounds answered 403; BotB entries carry no license deed | nothing that qualifies |
| YouTube, SoundCloud, Spotify | — | excluded by rule, not searched for files |

**Assessment.** Licensed Touhou chiptune recordings with a stable download
are scarce: one qualifying 8-bit track in the whole search (Misty Lake).
The licensed *data* route is not scarce: SUZUKI PLAN's 175 songs cover
every stage role. The CC chiptune fallback researched earlier (OpenGameArt
and others, non-Touhou) is not needed and is not in the manifest.

## 3. Pools per biome

Pools follow where the original plays in the games, so a biome sounds
like the matching part of a Touhou run (`internal/audio/tracks.toml`,
`pools` and `keys`). The topic biomes and boss rooms climb by act
(`State.ActOf`, `internal/app/audio.go`) as a run climbs by stage, so act
III never draws a stage 3 theme. A linear preset (short, patch) is one
act: its topic biome plays `act1`, its boss `boss1`.

| Pool | Plays in | Touhou role | Tracks |
|---|---|---|---|
| `grub` | GRUB menu, git log, top | title screens | 12 |
| `userland` | the first biome | stage 1-2 themes, plus Misty Lake | 22 |
| `act1` | act I's topic biomes | stage 3-4 themes, PoFV themes of early bosses | 23 |
| `act2` | act II's topic biomes | stage 5-6 and final stage themes, PoFV themes of stage 5 bosses | 19 |
| `act3` | act III's topic biomes | extra and phantasm stage themes, Last Word, Komachi | 12 |
| `boss1` | Boss I (Off-by-One, OOM Killer) | stage 1-3 boss themes | 27 |
| `boss2` | Boss II (Heisenbug, Race Condition, Undefined Behavior, the Court) | stage 4-5 and extra boss themes | 24 |
| `final` | The Halting Problem | final, extra and phantasm boss themes | 21 |
| `merge` | merge window | staff rolls and "Player's Score" | 8 |
| `cleared` | after the last boss | ending themes | 7 |
| `rtfm` | learn mode | ZUN's music-CD pieces and quiet unused tracks | 6 |
| `panic` | kernel panic | silent by default (`panic = ""` in audio.toml); one track if set | 1 |

PoFV's character themes have no stage, so each goes by its character's
stage as a boss in the first game that has them as one (Reimu and Marisa
LLS 4, Cirno EoSD 2, Prismriver PCB 4, Mystia IN 2, Aya MoF 4 → act I;
Sakuya, Youmu, Reisen, Tewi, Yuuka → act II). Medicine and Komachi are
bosses only in PoFV, so they go by where its story meets them: Medicine
act II, Komachi, the ferry to the Yama, act III. Taste, labelled as such.

Each boss opens with a theme of its own (`keys`); then its pool shuffles
as usual. A lead joins its boss's room from whatever pool it sits in
(`Library.Resolve`), so the short preset's one boss, in `boss1`, still
opens with Heisenbug's Koishi. Taste, labelled as such: Off-by-One gets
Cirno (⑨, who cannot count); Heisenbug gets Koishi and Nue (the
unobservable and the unidentifiable); OOM Killer gets Yuyuko (who devours
everything); Race Condition gets Sakuya (who stops time); Undefined
Behavior gets Flandre; the Court gets Alice's Doll Judgement (人形裁判: an
Alice, and a judgement); The Halting Problem gets Eiki (who decides every
case) and Yukari (who lives on the boundary).

A track plays `repeats` times (default 2) and the pool moves on without
repeats; songs without a loop mark, and Misty Lake, which ends in a decay,
play once a turn (`once`). The player's own files in
`~/.config/katharsis/music/<pool or topic or boss>/` (mp3, ogg, wav or
VGS mml) replace or extend a pool (`user_music`). That is also where the
Bandcamp albums above can go once the player has downloaded them.

The full lists:

#### grub (12)

| id | title | original |
|---|---|---|
| `th01-01` | A Sacred Lot | ZUN, Touhou 1 HRtP, Title screen |
| `th04-01` | Gensokyo　〜 Lotus Land Story (幻想郷　〜 Lotus Land Story) | ZUN, Touhou 4 LLS, Title screen |
| `th06-01` | a Dream that is more Scarlet than Red (赤より紅い夢) | ZUN, Touhou 6 EoSD, Title Screen theme |
| `th07-01` | Mystic Dream　〜 Snow or Cherry Petal (妖々夢　〜 Snow or Cherry Petal) | ZUN, Touhou 7 PCB, Title Screen theme |
| `th08-01` | Imperishable Night　〜 Eastern Night (永夜抄　〜 Eastern Night.) | ZUN, Touhou 8 IN, Title screen |
| `th09-01` | Kaeidzuka　〜 Higan Retour (花映塚　〜 Higan Retour) | ZUN, Touhou 9 PoFV, Title Screen theme |
| `th10-01` | Sealed Gods (封印されし神々) | ZUN, Touhou 10 MoF, Title screen |
| `th11-01` | Waking the Earth Spirits (地霊達の起床) | ZUN, Touhou 11 SA, Title screen |
| `th12-01` | Shadow of Blue Sky (青空の影) | ZUN, Touhou 12 UFO, Title screen |
| `th12_8-01` | Spring Ice Fairy (春の氷精) | ZUN, Touhou 12.8 Fairy Wars, Title theme |
| `th13-01` | The Greedy Spirits (欲深き霊魂) | ZUN, Touhou 13 TD, Title screen |
| `th14-01` | Mysterious Purified Stick (不思議なお祓い棒) | ZUN, Touhou 14 DDC, Title screen |

#### userland (22)

| id | title | original |
|---|---|---|
| `swapxfo-misty-lake` | Misty Lake (2A03+N163 Arrange) (ミストレイク) | ZUN, Touhou 14 DDC, Stage 1 theme |
| `th01-02` | Miko of Eternity (永遠の巫女) | ZUN, Touhou 1 HRtP, Theme of Levels 1-4, Hell (Jigoku) Levels 16-19 |
| `th04-02` | Witching Dream | ZUN, Touhou 4 LLS, Reimu Hakurei's Stage 1 theme |
| `th04-03` | Selene's Light | ZUN, Touhou 4 LLS, Marisa Kirisame's Stage 1 theme |
| `th04-05` | Break the Sabbath | ZUN, Touhou 4 LLS, Stage 2 theme |
| `th06-02` | a Soul Red like Ground Cherry (ほおずきみたいに紅い魂) | ZUN, Touhou 6 EoSD, Stage 1 theme |
| `th06-04` | Lunate Elf (ルーネイトエルフ) | ZUN, Touhou 6 EoSD, Stage 2 theme |
| `th07-02` | Paradise　〜 Deep Mountain (無何有の郷　〜 Deep Mountain) | ZUN, Touhou 7 PCB, Stage 1 theme |
| `th07-04` | the Fantastic Legend of Tohno (遠野幻想物語) | ZUN, Touhou 7 PCB, Stage 2 theme |
| `th08-02` | Illusionary Night　〜 Ghostly Eyes (幻視の夜　〜 Ghostly Eyes) | ZUN, Touhou 8 IN, Stage 1 theme |
| `th08-04` | Song of the Night Sparrow　〜 Night Bird (夜雀の歌声　〜 Night Bird) | ZUN, Touhou 8 IN, Stage 2 theme |
| `th10-02` | The gods that miss people　〜 Romantic Fall (人恋し神様　〜 Romantic Fall) | ZUN, Touhou 10 MoF, Stage 1 theme |
| `th10-04` | the Road of an Apotropaic God　〜 Dark Road (厄神様の通り道　〜 Dark Road) | ZUN, Touhou 10 MoF, Stage 2 theme |
| `th11-02` | The Wind Tunnel of Darkness (暗闇の風穴) | ZUN, Touhou 11 SA, Stage 1 theme |
| `th11-04` | The bridge with no survivors (渡る者の途絶えた橋) | ZUN, Touhou 11 SA, Stage 2 theme |
| `th12-02` | to the Springtime Harbor (春の湊に) | ZUN, Touhou 12 UFO, Stage 1 theme |
| `th12-04` | Closed Cloud Passage (閉ざせし雲の通い路) | ZUN, Touhou 12 UFO, Stage 2 theme |
| `th12_8-02` | Cute Great War Refrain (可愛い大戦争のリフレーン) | ZUN, Touhou 12.8 Fairy Wars, Stage 1 theme |
| `th12_8-04` | Year Round Preoccupied Curiosity (年中夢中の好奇心) | ZUN, Touhou 12.8 Fairy Wars, Stage 2 theme |
| `th13-02` | Deadly Night Cherry Blossoms (死霊の夜桜) | ZUN, Touhou 13 TD, Stage 1 theme |
| `th13-04` | Welcome to Yokai Temple (妖怪寺へようこそ) | ZUN, Touhou 13 TD, Stage 2 theme |
| `th14-02` | Mist Lake (ミストレイク) | ZUN, Touhou 14 DDC, Stage 1 theme |

#### act1 (23)

| id | title | original |
|---|---|---|
| `th04-07` | Bad Apple!! | ZUN, Touhou 4 LLS, Stage 3 theme |
| `th04-09` | Alice Maestra (アリスマエステラ) | ZUN, Touhou 4 LLS, Stage 4 theme |
| `th06-06` | Shanghai Teahouse　〜 Chinese Tea (上海紅茶館　〜 Chinese Tea) | ZUN, Touhou 6 EoSD, Stage 3 theme |
| `th06-08` | Voile，the Magic Library (ヴワル魔法図書館) | ZUN, Touhou 6 EoSD, Stage 4 theme |
| `th07-06` | the Doll Maker of Bucuresti (ブクレシュティの人形師) | ZUN, Touhou 7 PCB, Stage 3 theme |
| `th07-08` | the Capital City of Flowers in the Sky (天空の花の都) | ZUN, Touhou 7 PCB, Stage 4 theme |
| `th08-06` | Nostalgic Blood of the Eastern　〜 Old World (懐かしき東方の血　〜 Old World) | ZUN, Touhou 8 IN, Stage 3 theme |
| `th08-08` | Retribution for the Imperishable Night (永夜の報い　〜 Imperishable Night.) | ZUN, Touhou 8 IN, Stage 4 theme |
| `th09-02` | Spring Lane　〜 Colorful Path (春色小径　〜 Colorful Path) | ZUN, Touhou 9 PoFV, Reimu Hakurei's theme |
| `th09-03` | Oriental Dark Flight (オリエンタルダークフライト) | ZUN, Touhou 9 PoFV, Marisa Kirisame's theme |
| `th09-07` | Adventure of the Lovestruck Tomboy (おてんば恋娘の冒険) | ZUN, Touhou 9 PoFV, Cirno's theme; leads for `off-by-one` |
| `th09-08` | Phantom Ensemble (幽霊楽団　〜 Phantom Ensemble) | ZUN, Touhou 9 PoFV, Prismriver Sisters' (Lyrica, Merlin, and Lunasa) theme |
| `th09-09` | Unwittingly，Distracted with her Song　〜 Flower Mix (もう歌しか聞こえない　〜 Flower Mix) | ZUN, Touhou 9 PoFV, Mystia Lorelei's theme |
| `th09-11` | Fujin Maiden (ShortVersion) (風神少女 (ShortVersion)) | ZUN, Touhou 9 PoFV, Aya Shameimaru's theme |
| `th10-06` | the Gensokyo the Gods Loved (神々が恋した幻想郷) | ZUN, Touhou 10 MoF, Stage 3 theme |
| `th10-08` | Fall of Fall　〜 Autumnal Waterfall (フォールオブフォール　〜 秋めく滝) | ZUN, Touhou 10 MoF, Stage 4 theme |
| `th11-06` | Going along the Old Hell Road (旧地獄街道を行く) | ZUN, Touhou 11 SA, Stage 3 theme |
| `th11-08` | Heartfelt Fancy (ハートフェルトファンシー) | ZUN, Touhou 11 SA, Stage 4 theme |
| `th12-06` | Sky Ruins (スカイルーイン) | ZUN, Touhou 12 UFO, Stage 3 theme |
| `th12-08` | A Ghost Liner's Journey Through Time and Space (幽霊客船の時空を越えた旅) | ZUN, Touhou 12 UFO, Stage 4 theme |
| `th12_8-05` | Midnight Fairy Dance (真夜中のフェアリーダンス) | ZUN, Touhou 12.8 Fairy Wars, Stage 3 theme |
| `th13-06` | Let's Live in Nice Cemetery (素敵な墓場で暮しましょ) | ZUN, Touhou 13 TD, Stage 3 theme |
| `th13-08` | Desire Drive (デザイアドライブ) | ZUN, Touhou 13 TD, Stage 4 theme |

#### act2 (19)

| id | title | original |
|---|---|---|
| `th04-12` | Lotus Love | ZUN, Touhou 4 LLS, Stage 5 theme |
| `th06-10` | the Maid and the Pocket Watch of Blood (メイドと血の懐中時計) | ZUN, Touhou 6 EoSD, Stage 5 theme |
| `th06-12` | the Young Descendant of Tepes (ツェペシュの幼き末裔) | ZUN, Touhou 6 EoSD, Final Stage theme |
| `th07-10` | Mystic Oriental Dream　〜 Ancient Temple (東方妖々夢　〜 Ancient Temple) | ZUN, Touhou 7 PCB, Stage 5 theme |
| `th07-12` | Ultimate Truth (アルティメットトゥルース) | ZUN, Touhou 7 PCB, Final Stage theme |
| `th08-11` | Cinderella Cage　〜 Kagome-Kagome (シンデレラケージ　〜 Kagome-Kagome) | ZUN, Touhou 8 IN, Stage 5 theme |
| `th08-13` | Voyage 1969 (ヴォヤージュ1969) | ZUN, Touhou 8 IN, Final Stage theme |
| `th09-04` | Flowering Knight (フラワリングナイト) | ZUN, Touhou 9 PoFV, Sakuya Izayoi's theme; leads for `race-condition` |
| `th09-05` | Mystic Oriental Dream　〜 Ancient Temple (東方妖々夢　〜 Ancient Temple) | ZUN, Touhou 9 PoFV, Youmu Konpaku's theme |
| `th09-06` | Lunatic Eyes　〜 Invisible Full Moon (狂気の瞳　〜 Invisible Full Moon) | ZUN, Touhou 9 PoFV, Reisen Udongein Inaba's theme |
| `th09-10` | White Flag of the Usa Jingu (お宇佐さまの素い幡) | ZUN, Touhou 9 PoFV, Tewi Inaba's theme |
| `th09-12` | Poison Body　〜 Forsaken Doll (ポイズンボディ　〜 Forsaken Doll) | ZUN, Touhou 9 PoFV, Medicine Melancholy's theme |
| `th09-13` | Konjaku Gensokyo　〜 Flower Land (今昔幻想郷　〜 Flower Land) | ZUN, Touhou 9 PoFV, Yuuka Kazami's theme |
| `th10-10` | The Japanese Wilderness the Girl Saw (少女が見た日本の原風景) | ZUN, Touhou 10 MoF, Stage 5 theme |
| `th10-12` | Cemetery of Onbashira　〜 Grave of Being (御柱の墓場　〜 Grave of Being) | ZUN, Touhou 10 MoF, Stage 6 theme |
| `th11-12` | Hellfire Mantle (業火マントル) | ZUN, Touhou 11 SA, Stage 6 theme |
| `th12-10` | Esoteria, a local city in the magical world (魔界地方都市エソテリア) | ZUN, Touhou 12 UFO, Stage 5 theme |
| `th12-12` | Fire of the Dharmadhatu (法界の火) | ZUN, Touhou 12 UFO, Stage 6 theme |
| `th13-10` | Grand Enshrined Mausoleum at Yumedono (夢殿大祀廟) | ZUN, Touhou 13 TD, Stage 5 theme |

#### act3 (12)

| id | title | original |
|---|---|---|
| `th05-10` | Alice in the Wonderland (不思議の国のアリス) | ZUN, Touhou 5 MS, Extra Stage theme |
| `th06-14` | the Centennial Festival for Magical Girls (魔法少女達の百年祭) | ZUN, Touhou 6 EoSD, Extra Stage theme |
| `th07-15` | Charming Domination (妖々跋扈) | ZUN, Touhou 7 PCB, Extra Stage theme |
| `th07-17` | Charming Domination　〜 Who done it! (妖々跋扈　〜 Who done it!) | ZUN, Touhou 7 PCB, Phantasm Stage theme |
| `th08-17` | Extend Ash　〜 Hourai Victim (エクステンドアッシュ　〜 蓬莱人) | ZUN, Touhou 8 IN, Extra Stage theme |
| `th08-21` | Eastern Youkai Beauty (東方妖怪小町) | ZUN, Touhou 8 IN, Last Word theme |
| `th09-14` | Higan Homeward Voyage　〜 Riverside View (彼岸帰航　〜 Riverside View) | ZUN, Touhou 9 PoFV, Komachi Onozuka's theme |
| `th10-14` | Tomorrow will be Special，Yesterday was not (明日ハレの日、ケの昨日) | ZUN, Touhou 10 MoF, Extra stage theme |
| `th11-14` | Last Remote (ラストリモート) | ZUN, Touhou 11 SA, Extra stage theme |
| `th12-14` | UFO Romance in the Night Sky (夜空のユーフォーロマンス) | ZUN, Touhou 12 UFO, Extra stage theme |
| `th12_8-08` | Loose Rain (ルーズレイン) | ZUN, Touhou 12.8 Fairy Wars, Extra stage theme |
| `th13-14` | Yokai Backstreet (妖怪裏参道) | ZUN, Touhou 13 TD, Extra Stage theme |

#### boss1 (27)

| id | title | original |
|---|---|---|
| `th04-04` | Decoration Battle (装飾戦　〜 Decoration Battle) | ZUN, Touhou 4 LLS, Stage 1 Boss - Orange's theme |
| `th04-06` | Scarlet Symphony　〜 Scarlet Phoneme (紅響曲　〜 Scarlet Phoneme) | ZUN, Touhou 4 LLS, Stage 2 Boss - Kurumi's theme |
| `th04-08` | Spirit Battle　〜 Perdition Crisis (霊戦　〜 Perdition crisis) | ZUN, Touhou 4 LLS, Stage 3 Boss - Elly's theme |
| `th06-03` | Mystics Night (妖魔夜行) | ZUN, Touhou 6 EoSD, Stage 1 Boss - Rumia's theme |
| `th06-05` | Beloved Tomboyish Daughter (おてんば恋娘) | ZUN, Touhou 6 EoSD, Stage 2 Boss - Cirno's theme; leads for `off-by-one` |
| `th06-07` | Shanghai Alice in 1884 (明治十七年の上海アリス) | ZUN, Touhou 6 EoSD, Stage 3 Boss - Hong Meiling's theme |
| `th07-03` | Crystalize Silver (クリスタライズシルバー) | ZUN, Touhou 7 PCB, Stage 1 Boss - Letty Whiterock's theme |
| `th07-05` | Diao Ye Zong (Withered Leaf) (ティアオイエツォン（withered leaf）) | ZUN, Touhou 7 PCB, Stage 2 Boss - Chen's theme |
| `th07-07` | Doll Judgement (人形裁判　〜 人の形弄びし少女) | ZUN, Touhou 7 PCB, Stage 3 Boss - Alice Margatroid's theme; leads for `trial` |
| `th08-03` | Stirring an Autumn Moon　〜 Mooned Insect (蠢々秋月　〜 Mooned Insect) | ZUN, Touhou 8 IN, Stage 1 Boss - Wriggle Nightbug's theme |
| `th08-05` | Unwittingly，Distracted with her Song (もう歌しか聞こえない) | ZUN, Touhou 8 IN, Stage 2 Boss - Mystia Lorelei's theme |
| `th08-07` | Plain Asia (プレインエイジア) | ZUN, Touhou 8 IN, Stage 3 Boss - Keine Kamishirasawa's theme |
| `th09-07` | Adventure of the Lovestruck Tomboy (おてんば恋娘の冒険) | ZUN, Touhou 9 PoFV, Cirno's theme; leads for `off-by-one` |
| `th10-03` | Because Princess Inada is Scolding Me (稲田姫様に叱られるから) | ZUN, Touhou 10 MoF, Stage 1 Boss - Minoriko Aki's theme |
| `th10-05` | Darkside of Fate (運命のダークサイド) | ZUN, Touhou 10 MoF, Stage 2 Boss - Hina Kagiyama's theme |
| `th10-07` | Akutagawa Ryuunosuke's "Kappa"　〜 Candid Friend (芥川龍之介の河童　〜 Candid Friend) | ZUN, Touhou 10 MoF, Stage 3 Boss - Nitori Kawashiro's theme |
| `th11-03` | Sealed Yokai　〜 Lost Place (封じられた妖怪　〜 Lost Place) | ZUN, Touhou 11 SA, Stage 1 boss - Yamame Kurodani's theme |
| `th11-05` | Green-Eyed Jealousy (緑眼のジェラシー) | ZUN, Touhou 11 SA, Stage 2 boss - Parsee Mizuhashi's theme |
| `th11-07` | Petals sake cup Ohe Mountain (華のさかづき大江山) | ZUN, Touhou 11 SA, Stage 3 boss - Yuugi Hoshiguma's theme |
| `th12-03` | Tiny Little Clever Commander (小さな小さな賢将) | ZUN, Touhou 12 UFO, Stage 1 Boss - Nazrin's theme |
| `th12-05` | Beware of the Ever-Present Umbrella (万年置き傘にご注意を) | ZUN, Touhou 12 UFO, Stage 2 Boss - Kogasa Tatara's theme |
| `th12-07` | Outdated Father and High-Collar Girl (時代親父とハイカラ少女) | ZUN, Touhou 12 UFO, Stage 3 Boss - Ichirin Kumoi and Unzan's theme |
| `th12_8-03` | Risking Life for the Sake of Mischief (いたずらに命をかけて) | ZUN, Touhou 12.8 Fairy Wars, Stage 1/2 boss theme |
| `th12_8-06` | Fairy Wars　〜 Faily Wars (妖精大戦争　〜 Faily Wars) | ZUN, Touhou 12.8 Fairy Wars, Stage 3 boss theme |
| `th13-03` | Ghost Lead (ゴーストリード) | ZUN, Touhou 13 TD, Stage 1 Boss - Yuyuko Saigyouji's theme; leads for `oom-killer` |
| `th13-05` | Yokai Girl who at the Gate (門前の妖怪小娘) | ZUN, Touhou 13 TD, Stage 2 Boss - Kyouko Kasodani's theme |
| `th13-07` | Rigid Paradise (リジッドパラダイス) | ZUN, Touhou 13 TD, Stage 3 Boss - Yoshika Miyako's theme |

#### boss2 (24)

| id | title | original |
|---|---|---|
| `th01-03` | The Positive and Negative | ZUN, Touhou 1 HRtP, Shingyoku's theme (stage 5 boss) |
| `th04-10` | Maiden's Capriccio (少女綺想曲　〜 Capriccio) | ZUN, Touhou 4 LLS, Stage 4 Boss A - Reimu Hakurei's theme |
| `th04-11` | Casket of Star (星の器　〜 Casket of Star) | ZUN, Touhou 4 LLS, Stage 4 Boss B - Marisa Kirisame's theme |
| `th04-13` | Sleeping Terror (眠れる恐怖　〜 Sleeping Terror) | ZUN, Touhou 4 LLS, Stage 5 Boss - Yuuka Kazami's 1st theme |
| `th06-09` | Locked Girl　〜 the Girl's Secret Room (ラクトガール　〜 少女密室) | ZUN, Touhou 6 EoSD, Stage 4 Boss - Patchouli Knowledge's theme |
| `th06-11` | Luna Dial (月時計　〜 ルナ・ダイアル) | ZUN, Touhou 6 EoSD, Stage 5 Boss - Sakuya Izayoi's theme; leads for `race-condition` |
| `th06-15` | U.N.Owen was her? (U.N.オーエンは彼女なのか？) | ZUN, Touhou 6 EoSD, Extra Boss - Flandre Scarlet's theme; leads for `undefined-behavior` |
| `th07-09` | Phantom Ensemble (幽霊楽団　〜 Phantom Ensemble) | ZUN, Touhou 7 PCB, Stage 4 Boss - Prismriver Sisters' (Lunasa, Merlin, and Lyrica) theme |
| `th07-11` | Hiroari Shoots a Strange Bird　〜 Till When? (広有射怪鳥事　〜 Till When?) | ZUN, Touhou 7 PCB, Stage 5 Boss - Youmu Konpaku's theme |
| `th07-13` | Bloom Nobly，Ink-black Cherry Blossom (幽雅に咲かせ、墨染の桜　〜 Border of Life) | ZUN, Touhou 7 PCB, Final Boss - Yuyuko Saigyouji's theme; leads for `oom-killer` |
| `th08-09` | Maiden's Capriccio　〜 Dream Battle (少女綺想曲　〜 Dream Battle) | ZUN, Touhou 8 IN, Stage 4 uncanny Boss - Reimu Hakurei's theme |
| `th08a-10` | Love Colors Master Spark (恋色マスタースパーク) | ZUN, Touhou 8 IN, Stage 4 powerful Boss - Marisa Kirisame's theme |
| `th08-12` | Lunatic Eyes　〜 Invisible Full Moon (狂気の瞳　〜 Invisible Full Moon) | ZUN, Touhou 8 IN, Stage 5 Boss - Reisen Udongein Inaba's theme |
| `th09-04` | Flowering Knight (フラワリングナイト) | ZUN, Touhou 9 PoFV, Sakuya Izayoi's theme; leads for `race-condition` |
| `th10-09` | The Youkai Mountain　〜 Mysterious Mountain (妖怪の山　〜 Mysterious Mountain) | ZUN, Touhou 10 MoF, Stage 4 boss - Aya Shameimaru's theme |
| `th10-11` | Faith is for the Transient People (信仰は儚き人間の為に) | ZUN, Touhou 10 MoF, Stage 5 boss - Sanae Kochiya's theme |
| `th11-09` | Satori Maiden　〜 3rd eye (少女さとり　〜 3rd eye) | ZUN, Touhou 11 SA, Stage 4 boss - Satori Komeiji's theme |
| `th11-11` | Corpse Voyage　〜 Be of good cheer! (死体旅行　〜 Be of good cheer!) | ZUN, Touhou 11 SA, Stage 5 boss - Rin Kaenbyou's theme |
| `th11-15` | Hartmann's Specter Girl (ハルトマンの妖怪少女) | ZUN, Touhou 11 SA, Extra stage boss - Koishi Komeiji's theme; leads for `heisenbug` |
| `th12-09` | Captain Murasa (キャプテン・ムラサ) | ZUN, Touhou 12 UFO, Stage 4 Boss - Murasa Minamitsu's theme |
| `th12-11` | Bishamonten with Tiger Pattern (虎柄の毘沙門天) | ZUN, Touhou 12 UFO, Stage 5 Boss - Shou Toramaru's theme |
| `th12-15` | Alien in Heian (平安のエイリアン) | ZUN, Touhou 12 UFO, Extra Stage Boss - Nue Houjuu's theme; leads for `heisenbug` |
| `th13-09` | Ancient Yuan-Sheng (古きユアンシェン) | ZUN, Touhou 13 TD, Stage 4 Boss - Seiga Kaku's theme |
| `th13-11` | Story of Mythomiwa (大神神話伝) | ZUN, Touhou 13 TD, Stage 5 Boss - Mononobe no Futo's theme |

#### final (21)

| id | title | original |
|---|---|---|
| `th05-11` | the Grimoire of Alice | ZUN, Touhou 5 MS, Extra Boss - Alice Margatroid's 2nd theme |
| `th06-13` | Septette for the Dead Princess (亡き王女の為のセプテット) | ZUN, Touhou 6 EoSD, Final Boss - Remilia Scarlet's theme |
| `th06-15` | U.N.Owen was her? (U.N.オーエンは彼女なのか？) | ZUN, Touhou 6 EoSD, Extra Boss - Flandre Scarlet's theme; leads for `undefined-behavior` |
| `th07-13` | Bloom Nobly，Ink-black Cherry Blossom (幽雅に咲かせ、墨染の桜　〜 Border of Life) | ZUN, Touhou 7 PCB, Final Boss - Yuyuko Saigyouji's theme; leads for `oom-killer` |
| `th07-14` | Boarder of Life (ボーダーオブライフ) | ZUN, Touhou 7 PCB, Final Boss - Yuyuko Saigyouji's revival theme |
| `th07-16` | a Maiden's Illusionary Funeral　〜 Necro-Fantasy (少女幻葬　〜 Necro-Fantasy) | ZUN, Touhou 7 PCB, Extra Boss - Ran Yakumo's theme |
| `th07-18` | Necro Fantasia (ネクロファンタジア) | ZUN, Touhou 7 PCB, Phantasm Boss - Yukari Yakumo's theme; leads for `halting-problem` |
| `th08-14` | Gensokyo Millennium　〜 History of the Moon (千年幻想郷　〜 History of the Moon) | ZUN, Touhou 8 IN, FinalA Boss - Eirin Yagokoro's theme |
| `th08-15` | Flight of the Bamboo Cutter　〜 Lunatic Princess (竹取飛翔　〜 Lunatic Princess) | ZUN, Touhou 8 IN, FinalB Boss - Kaguya Houraisan's theme |
| `th08-16` | Voyage 1970 (ヴォヤージュ1970) | ZUN, Touhou 8 IN, Eirin and Kaguya's Revival theme |
| `th08-18` | Reach for the Moon，Immortal Smoke (月まで届け、不死の煙) | ZUN, Touhou 8 IN, Extra Boss - Fujiwara no Mokou's theme |
| `th09-15` | Eastern Judgement　〜 Fate of Sixty Years (六十年目の東方裁判　〜 Fate of Sixty Years) | ZUN, Touhou 9 PoFV, Shikieiki Yamaxanadu's theme; leads for `halting-problem` |
| `th10-13` | VenerableAncientBattlefield 〜 SuwaFoughtenField (神さびた古戦場　〜 Suwa Foughten Field) | ZUN, Touhou 10 MoF, Stage 6 boss - Kanako Yasaka's theme |
| `th10-15` | Native Faith (ネイティブフェイス) | ZUN, Touhou 10 MoF, Extra stage boss - Suwako Moriya's theme |
| `th11-13` | Heliolithic of Mystic Wisdom　〜 Nuclear Fusion (霊知の太陽信仰　〜 Nuclear Fusion) | ZUN, Touhou 11 SA, Stage 6 boss - Utsuho Reiuji's theme |
| `th11-15` | Hartmann's Specter Girl (ハルトマンの妖怪少女) | ZUN, Touhou 11 SA, Extra stage boss - Koishi Komeiji's theme; leads for `heisenbug` |
| `th12-13` | Emotional Skyscrapers　〜 Cosmic Mind (感情の摩天楼　〜 Cosmic Mind) | ZUN, Touhou 12 UFO, Stage 6 Boss - Byakuren Hijiri's theme |
| `th12-15` | Alien in Heian (平安のエイリアン) | ZUN, Touhou 12 UFO, Extra Stage Boss - Nue Houjuu's theme; leads for `heisenbug` |
| `th12_8-09` | Magus Night (メイガスナイト) | ZUN, Touhou 12.8 Fairy Wars, Extra boss theme |
| `th13-13` | Shotoku Legend　〜 True Administrator (聖徳伝説　〜 True Administrator) | ZUN, Touhou 13 TD, Stage 6 Boss - Toyosatomimi no Miko's theme |
| `th13-15` | Futatsuiwa from Sado (佐渡の二ッ岩) | ZUN, Touhou 13 TD, Extra Stage Boss - Mamizou Futatsuiwa |

#### merge (8)

| id | title | original |
|---|---|---|
| `th06-17` | Crimson Belvedere　〜 Eastern Dream... (紅楼　〜 Eastern Dream...) | ZUN, Touhou 6 EoSD, Staff Roll theme |
| `th07-20` | Sakura Sakura　〜 Japanize Dream... (さくらさくら　〜 Japanize Dream...) | ZUN, Touhou 7 PCB, Staff Roll theme |
| `th08-20` | Eternal Dream　〜 Mystic Maple (Eternal Dream　〜 幽玄の槭樹) | ZUN, Touhou 8 IN, Staff Roll theme |
| `th10-17` | the Gods give us Blessed Rain　〜 Sylphid Dream (神は恵みの雨を降らす　〜 Sylphid Dream) | ZUN, Touhou 10 MoF, Staff roll theme |
| `th10-18` | Player's Score (プレイヤーズスコア) | ZUN, Touhou 10 MoF, Score theme |
| `th11-17` | Energy Dawn 〜 Future Dream... (エネルギー黎明 〜 Future Dream...) | ZUN, Touhou 11 SA, Staff roll theme |
| `th12-17` | Way Home in the Sky　〜 Sky Dream (空の帰り道　〜 Sky Dream) | ZUN, Touhou 12 UFO, Staff roll theme |
| `th12_8-10` | Players Score (プレイヤーズスコア) | ZUN, Touhou 12.8 Fairy Wars, Score theme |

#### cleared (7)

| id | title | original |
|---|---|---|
| `th06-16` | Crimson Beyond a Fleeting Eternity (紅より儚い永遠) | ZUN, Touhou 6 EoSD, Ending theme |
| `th07-19` | Dream of a Spring Breeze (春風の夢) | ZUN, Touhou 7 PCB, Ending theme |
| `th08-19` | Evening Primrose (月見草) | ZUN, Touhou 8 IN, Ending theme |
| `th10-16` | Shrine at the Foot of the Mountain (麓の神社) | ZUN, Touhou 10 MoF, Ending theme |
| `th11-16` | The Return of the Earth Spirits (地霊達の帰宅) | ZUN, Touhou 11 SA, Ending theme |
| `th12-16` | Yokai Temple (妖怪寺) | ZUN, Touhou 12 UFO, Ending theme |
| `th12_8-07` | Spring Ice Fairy - Silent - (春の氷精 - 静 -) | ZUN, Touhou 12.8 Fairy Wars, Ending theme |

#### rtfm (6)

| id | title | original |
|---|---|---|
| `th01-16` | Wind Shinto Shrine (風の神社) | ZUN, Touhou 1 HRtP, unused HRtP track |
| `zsw-01` | Magician of the Evening Darkness (宵闇の魔術師) | ZUN, ZUN's Strange Works (CD), ZUN's Strange Works (music CD, not a game) |
| `zsw-03` | Plastic Space | ZUN, ZUN's Strange Works (CD), ZUN's Strange Works (music CD) |
| `zsw-06` | Hourai Illusion　〜 far East (蓬莱幻想　〜 far East) | ZUN, ZUN's Strange Works (CD), ZUN's Strange Works (music CD) |
| `hourai-01` | Legend of Hourai (蓬莱伝説) | ZUN, Dolls in Pseudo Paradise (CD), Dolls in Pseudo Paradise (music CD) |
| `th07-21` | Flower of Past Days　〜 Fairy of Flower (過去の花　〜 Fairy of Flower) | ZUN, listed with PCB in 東方BGM on VGS |

#### panic (1)

| id | title | original |
|---|---|---|
| `th11-10` | Lullabies of an Abandoned Hell (廃獄ララバイ) | ZUN, Touhou 11 SA, Stage 5 theme |

## 4. Engine

**Choice: ebitengine/oto v3 with katharsis's own mixer.** oto v3 reaches
CoreAudio through purego, so the build stays `CGO_ENABLED=0` (the flake
builds that way). gopxl/beep plays through oto as well and would only
add its own mixer on top; katharsis needs crossfades, ducking, a jingle
queue and a limiter it can test without a device, so the mixer is
`internal/audio/mixer.go` and beep is not used. Decoders: the VGS port
(`internal/audio/vgs`), hajimehoshi/go-mp3 (with LAME gapless trim) and
jfreymuth/oggvorbis; WAV is parsed in-house. afplay is gone.

Numbers (macOS arm64, Go 1.26.8, `-trimpath -ldflags "-s -w"`,
`CGO_ENABLED=0`):

| | |
|---|---|
| binary before audio | 28,394,018 B |
| binary now | 29,339,074 B (+945,056 B, +3.3%) |
| of which oto + go-mp3 + oggvorbis | +357,280 B (beep instead: +408,752 B) |
| of which VGS tone tables | about 227 KB (113,553 int16 samples) |
| idle, device open, nothing playing | 2.2% of one core (40 ms device buffer) |
| idle, device suspended | 0.16% of one core; the engine suspends after 3 s with nothing to play (`idle_suspend`) |
| resume after suspend | 30 µs |
| playing a VGS song (render, resample, mix, limit) | 0.37% of one core |
| playing an MP3 | 2.8% of one core |
| audio buffered ahead of the device | 32 ms average, 42 ms max, plus the 40 ms device buffer |
| effect latency, Play to the speaker | about 70 ms (*inferred*: the two buffers above) |
| device open at start | 0.4-1.2 s, in the background; effects before it are dropped, not played late |

Idle and latency were measured by a spike that streamed digital silence
(every sample asserted zero), so nothing was heard; playing CPU was
measured by rendering 60 s through the mixer with no device at all.

**VGS port.** `internal/audio/vgs` is a Go port of vgsmml.c, vgsdec.c and
vgs2tone.c from tohovgs-cli `cbb13ad`. It keeps the C quirks the songs
were tuned against (per-channel clipping, a channel's tracked volume
starting at 0, integer gate arithmetic) and drops what the songs never
use (FM "kobushi" modulation, fade-out, the threaded queue, zlib between
compiler and decoder). `TestConformance` compares it with a C build of
the originals (`internal/audio/vgs/testdata/ref.c`):

```
VGS_REF=/tmp/kaudio/vgsref/ref.txt go test -run Conformance -v ./internal/audio/vgs
    vgs_test.go: 176 songs match the C decoder sample for sample
```

The hash covers each song's first pass and 10 s past its loop point, so
the jump back is checked too. Songs render at 22050 Hz mono, as the C
decoder does, and the stream resamples to 44.1 kHz (4-point Hermite).

**Loops.** A VGS song carries its loop mark (`|`); at the end of the first
pass the decoder carries on from it, and the stream's seek back is that
same continuation, so the seam is the arranger's own, sample exact
(`TestMMLLoopsSeamlessly`). Recordings loop on `loop_start`/`loop_end` or
the whole file.

## 5. Delivery

**`katharsis audio fetch`, not first use.** A timed game should not start
downloads on its own: the player chooses when the network is used, and
the game never waits on it. `fetch` downloads every track once, from each
track's own source, into `~/Library/Caches/katharsis/music`, checks the
pinned SHA-256 before an atomic rename (temp file + `rename`), prints
progress, retries twice, waits 0.5 s between requests, and measures each
file's loudness once into `loudness.json`. Without the files, offline or
before a fetch, music is silence and effects still play. `katharsis audio
list` shows what is downloaded; `katharsis audio credits` prints the
credits page.

Run in a temporary `HOME`: 176 of 176 fetched, 0 failed, 5.0 MiB, 2 min
4 s wall (most of it the 0.5 s gaps).

The User-Agent is `katharsis/0.1 (terminal game; downloads each music file
once and caches it)`; no request carries any identity of the player.

## 6. Effects and levels

Effects are synthesized at start from `internal/audio/sfx.toml` (pulse,
triangle, saw and noise tones, sfxr-style), so they carry no license:
`ac`, `wa`, `crash`, `commit`, `boss`, `cleared`, `door` (a door opens),
`submit` (judging starts), `life` (a life lost), `warn` (60 s left),
`buy` and `reroll` (merge window), `pick`, `prestige` (an unlock), and
`quack`, a literal duck. All are 0.1-1.05 s.

How the levels were chosen:

- **Music at -20 LUFS** (integrated, BS.1770) at volume 1: background
  level, well under streaming loudness (-14 to -16 LUFS), because the game
  runs beside the player's own thinking for hours. VGS songs measure
  -11.0 to -6.2 LUFS (mean -8.2), so they play about 12 dB down, and their
  peaks land far under the limiter.
- **Effects at -16 LUFS** for a verdict (loudest 400 ms), 4 LU over the
  music so a verdict is heard over it; routine effects sit lower (`level`
  in sfx.toml: door -20, submit -22, pick and reroll -21), the big moments
  higher (crash, cleared, prestige -15). Measured peaks are -6.1 dBFS or
  lower; the synth caps them at -1 dBFS (`TestEffects`).
- **Volumes on a squared taper** (gain = v²): the defaults 0.7 music and
  0.8 effects are -6.2 dB and -1.9 dB.
- **Ducking:** music drops 6 dB (`duck_gain` 0.5) while a submission
  judges, over 150 ms, and returns over 800 ms.
- **Limiter:** linear to ±0.9, then a tanh knee that never reaches ±0.98,
  so a pile-up saturates instead of clipping.

Measured on the VGS songs, at their own level before any gain: 0.04% of
samples clip in the C decoder on average (one song, TH06-02, at 0.45%);
that is the arranger's sound and is kept, since the port must match the C
output.

All numbers are settings in `~/.config/katharsis/audio.toml` (D2).

## 7. How the manifest was made

1. The song list is tohovgs-cli's `mml/songlist.json`; each MML file was
   downloaded at `cbb13ad`, hashed with `shasum -a 256` and sized.
2. Where each original plays came from the Touhou Wiki's music pages
   (touhou.fandom.com wikitext, `{{lang}}` markup stripped); titles in
   English are SUZUKI PLAN's own (`english` in songlist.json).
3. Pools and leads were assigned by role (section 3) and joined into
   `tracks.toml` with an awk script; durations are each song's first pass
   as the Go port times it, which matches the C decoder's count for every
   song.

## 8. Calls for the user

1. **Label katharsis a Touhou Project fan work.** SUZUKI PLAN's permission
   covers "東方Projectの二次創作物". The credits page, `audio fetch` and the
   README now say so; if the game should not carry that label, the VGS
   music cannot stay.
2. **Where the game is distributed.** ZUN's guideline limits fan works to
   channels where Touhou is sold or that he permits; the Japanese list
   names indie game sites. GitHub is not listed. Nothing is distributed
   yet.
3. **Optional courtesy:** fork `suzukiplan/Touhou-VGS-MML-data` on GitHub,
   which satisfies the 2016 wording too, and tell SUZUKI PLAN about the
   game ("ご連絡いただければ喜びます"). Both need the user's own account.
4. **Bandcamp albums** (+TEK's CC BY FM albums especially) are licensed for
   this use but can only be downloaded by hand; the player can drop them
   into `~/.config/katharsis/music/<pool>/`.
5. **The kernel panic is silent** by default; `panic = "panic"` plays
   Lullaby of Deserted Hell there instead.
6. **Lead themes per boss** are taste (section 3); they are one line each
   in `tracks.toml`.
