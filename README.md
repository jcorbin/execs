# Extreme ECS Programming (in GoLang)

This repository exists to explore building an Entity Component System in Go.

Each branch is an orphan containing an attempt.

## The Road So Far

### [One](../../tree/one)

My first attempt didn't get so far. Coasting on memory of reading blog posts, I
dove in and tried to scale Abstraction mountain. This went about as well as
you'd expect for a low level character. Fortunately I only spent about an hour
on this attempt.

### [Two](../../tree/two)

With the sting of death fresh in my nerves, I went back to the blog posts. I
picked one that seemed ruthlessly simple; in fact I picked exactly BECAUSE I
could already see some of its limitations.

This turned out to be a good choice, since what I was really lacking in attempt
one was any developed "muscles" for things like:
- terminal manipulation
- event handling
- what even is a good boundary to draw around a `System`?

Attempt two ended up being "playable":
- single player, random rolled stats
- a room outline
- AIs (demons) keep spawning, also with random rolled stats
  - spawn chance is a 1/N (where N is the number of combatants)

Meta progress:
- I plan to re-use parts of Two's code for Three; in particular, the drudgery
  of Yet Another Point Struct
- in Three I plan to start with the rendering system first rather than from the
  movement/combat system as I did with Two

### [Three](../../tree/three)

Feeling exhilarated from yesterday's progress, I took a day to focus on
building a home:

- `internal/point/point.go`: a cleaned up version of Two's `main.Point` for re-use.
- `internal/view/grid.go`: a `Point` + `termbox.Cell` structure, called `Grid`,
  with some of the rendering code from Two for re-use: it can copy one grid
  into another (centering or clamping as needed), and it can write a string
  into the grid (aligned left, center, or right).
- `internal/view/view.go`: consolidates most of the termbox code from Two into
  `View` which provides: an event polling loop, key event channel, and
  rendering system based on a `Grid`, header, footer, and logs.
- `internal/ecs/core.go`: defines the `Core` of an ECS, which you can embed
  into e.g. `main.World` struct. I'm coming to understand that this might be
  called an ["EntityManager" by the 'RDBMS Beta' interpretation][es-beta].
- `internal/ecs/entity.go`: provides what the [Beta interpretation][es-beta]
  might call a "MetaEntity": a convenience wrapper/reference to an entity for
  use outside of the ECS itself (e.g. by your main wiring code).

The actual "game" this time around is more demo: it statically renders what was
originally intended to be a "snake", but turned out to be more of a "hallway
system" since it's allowed to double back on itself.

#### [Three Prime](../../tree/three_prime)

Not being satisfied with my lovely new home, I set out to explore more: I
wanted to put my "not-quite-a-snake" in a "room", and experiment with
representing walls using compact tiles for a change. That may have worked, but
I also decided that boring straight-walled rooms were so yesterday, and instead
set out to draw a "permuted" room.

Well one thing lead to another, and before I knew it dusk was setting in as I
was trying to debug a complex digger that used an engineered markov transition
table to control displacement within a "toroidal" space... I never got it out
of "slice index range error panic" hell, but the partial result is still
interesting enough for post-mortem (I did end up adding dot product to my
2-vectors after all...)

[es-beta]: http://entity-systems.wikidot.com/rdbms-beta
