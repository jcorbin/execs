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
building a [home](../../tree/home):

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

### [Four](../../tree/four)

Decided to continue focusing on home improvements for now, especially since I
keep wrestling with debugging the view layer that I keep carrying along. Didn't
reach full satisfaction with it today, but I did at least get Two's world
ported over to it:

- many improvements to the point package, including a box defined by a top-left
  and bottom-right corners
- added iteration support to the ECS
- added a skeleton main.go inside the ECS module for faster start-over
- started reworking the view layer to suck less; learned a lot more about how I
  wish termbox was different (mostly simpler and more low level...)

### [Five](../../tree/five)

Finished off the view work, I now have it in a better place. I now have a
decent internal grudge list against termbox, and look forward excitedly to a
better/simpler/more-low-level alternative.

The two big items:
- added a markov transition table package: it allows you to markov chain
  through any ECS's entity space!
- ported forward Four's world to the new view code, and started using colors!
  - player and enemy colors now darken with lost HP
  - the floor and walls now have a randomized texture generated by colors
    chosen from a markov table
- oh right, and corpses now stick around; a great jumping off point for
  things to come!

And here's some other minor notes:
- added view support for merging into terminal grid cells; e.g. BG color
  provided by one update, glyph by another
- finished the view layer refactor, now things work a lot better, including
  resize works properly!
- entity iteration improved, and simplified in implementation detail
- entity references got way more useful
- iterated on the skeleton main.go for new view situation

### [Six](../../tree/six)

Today I dove back into the depths of ECS land. Having glimpsed possibilities
when reading [about the Beta approach][es-beta], and being plussed by how well
the markov table part went, I set out to build a "body" system:
- no more "arbitrary" Str, Def, Dex, and Luck; instead each world entity can have a body
- a body is itself an ECS composed of body parts; each body part has hp, a
  damage rating, and an armor rating
- the body parts exist in a hierarchical relation ship, such that if your arm
  is destroyed, so is your hand; this makes it possible to kill by destroying
  the head or torso
- severed body parts now drop into the world as separate item entities
- death now happens when you're "disembodied": when all body parts have dropped
  into the world, leaving an entity's body empty

For bonus, I made it so that if your head is still intact (i.e. you were killed
by torso destruction), then you get to continue on as a spirit for 5 turns.
Spirits can still collide, but since combat is based on presence of a body, no
damage happens (this happens to enemies as well of course).

Combat sub-targets are chosen automatically for you, but as soon as I level up
the view controller enough to support it, I'll add a mini-game allowing the
player to choose (e.g. to attack a leg, instead of going for the kill).

Next up I have plans for all those spare body parts... also maybe reclaim
something from destroyed body parts (e.g. the remaining damage/armor points on
whatever body part(s) you destroy)...

### [Seven](../../tree/seven)

Seven is an auspicous time to start over; now it's starting to look a lot more
like a base of data!

So yesterday's adventure got really bogged down in debugging land: all of my
inline relation and graph management code in the main package turned out to be
way more brittle than you'd hope.

Today I started out on a new orphan branch writing an ECS again from scratch!
Some high points:
- entity ids now have an explicit type, no longer just an `int`
- lifecycle hooks are now provided for data allocation, creation, and destruction
- a relational core extension is provided: its also a core, and its entities
  decribe relations between two other cores (maybe the same ones)
- the case of an "auto-relation" can further be upgraded to a graph relation;
  this is just a relational core where both the A and B cores are the same;
  currently not much exciting other than a `Roots` and `Leaves` method have
  been implemented.

I was able to get through that initial re-implementation in about 5 hours, I
then spent the next 5 hours porting Six to the new ECS engine. Needless to say,
it's working much better:
- the collision system is now just a relation table...
- ...so it'd be possible to use it to store a per-round damage / kill data as
  well
- I ported the markov table, but there's some consideration to be given to
  could it be "just" a relational table? Probably, but the current markov table
  is likely more space efficient than doing it that way.

I'm excited for: Eight: I'll either work further down to deepen the body
system, or expand out and start adding an agro table... only time will tell!

### [Eight](../../tree/eight)

Refactoring my way towards anger management:
- so progress on the ECS core:
  - Relation has now been much more tested and works well
  - Graph learned how to do DFS and CoDFS traversal
  - Cursor grew up, now supports indices
  - Cores now support generic create/destroy hooks
  - Relation uses those hooks to destroy orphaned relations...
  - ...and to provide optional cascading destroy
  - Core now plays it close to the chest
  - there's test coverage.

And for all that refactoring, the main game now:
- has an "agro" relation, which will cause the ai to chase the thing it hates most
- a "goal" relation which the ai will chase if it's got nothing to hate
- if ai doesn't have a goal, it picks a random non-combatant collidable
- since there's not yet any source of agro (i.e. damage should, maybe kills
  too), that's it

So in net: the AIs now mill around and kill each other, while the player can
play it safe from the edge; Whew!

### [Nine](../../tree/nine)

Small improvements and completing things:

- ecs progress:
  - several small fixes and improvements to Relation and friends
  - some parts got more debug friendly (fmt.Stringer implementations)
- world progress:
  - closed the agro loop: damage deals agro
  - added ai goal unsticking so that they stop getting stuck on the wall
    indefinitely
  - move generation decoupled from move applying; this leaves room for more
    mature move resolution strategies than first-one-wins
  - many cleanups and improvements to control flow

### [Ten](../../tree/ten)

Prompts, Interaction, and Combat:

- ECS progress:
  - improved where function convention
  - made Update's set function more useful/consistent
  - more minor conveniences

- Game Progress:
  - continued to improve the move processing
  - added more body parts (legs and arms now have an upper and lower part)
  - reworked the combat system to be less tedious:
    - potential damage is now `Round(dmg * rating * randBetween(0.5, 1.0))`, where:
    - `dmg` is the attacking part's damage score
    - `rating` is `srcHPRating / targHPRating`
    - `rand` is a random number in the internal `[0.5, 1.0)`
    - each `HPRating` is a hierarchical average rating of `hp/maxHP` from the
      attacking part all the way up its control path (i.e. rating compounding
      goes down for damage along any upstream part)
    - after potential damage, comes armor mitigation
    - misses are no longer a thing; may bring them back at some point once
      movement is taken into account during combat
  - started sketching an item interaction system:
    - spent most of my time building out a menu-based prompt system
    - body remains can now be looted for armor and damage points

- Misc:
  - added a `moremath.Round` utility

### [Eleven](../../tree/eleven)

Getting more out of your remains (but be quick!)

Today's focus was on the game, such as it is; no progress made in ECS land
itself:
- completed the item interaction system
- the first interaction is integrating a body part's armor and damage points
  into your own; doing so consumes a turn
- AI can now use items (and it prefers to do so when choosing a goal)
  - it uses the same prompting system that the user does, choosing randomly
- body remains now decay rapidly; also spirit duration is now coupled to the
  head from whence it came!
- added standard rogue-like vikeys
- added a quick hack to "phase" the palyer out of the collision system, mostly
  because this aids developing since I can just "phase out, and let the enemies
  have at it" for a while to see how things play out

### [Twelve](../../tree/twelve)

I'm really starting to get a charge out of this!

So I started out "just" wanting to make movement scale with leg health.  Well
as soon as I had that, the immediate next was "okay, so how could you still
move 'slower' when your leg system <50%?"  The 'obvious' answer was "well, you
just have to keep trying, and maybe you'll move every-other-turn if you do!"

So I re-purposed the mechanism that the AI uses to unstick itself: increment a
counter on the pending moves relation each time, so that if a move stays
pending, it accumulates a higher count.  The move application logic just takes
that count into effect as a multiplier.  Well the immediate side-effect of this
was: "cool story, so now when you're at 100%, you can move N squares at a time
by just staying in place for N turns."

This obviously needed to be made A Thing, so I called it `Charge`:
- for now the maximum `Charge` that you can accumulate is `4`
- you can only apply up to two `Charge` points each turn to a move (so you
  can't jump arbitrarily far)
- furthermore, when you attack with `Charge`, it also serves as a damage
  multiplier (for now all `Charge` is absorbed by combat)

Changes:
  - Game Mechanics:
    - made spirits not collide
    - made AI prefer closer items
    - the floor now starts out much cleaner, but decayed remains will leave marks!
    - movement now becomes difficult to impossible when your legs are damaged...
    - ...to help compensate, there is now a new Charge mechanic: with Charge you
      can move faster when healthy, and at all when damaged; Charge also
      increases damage dealt.
    - fixed items-here collision logic
    - fixed rare AI goal choice bug
  - Game UI:
    - dropped quite a bit of log spam
    - fixed an uncommon crash when resizing the terminal
    - the header and footer now have fancy left/right alignment support
    - key handling got way better to support the new action bar
    - the "Items Here" prompt is now a dynamic action bar item...
    - ...for the purists, there's now a `","` binding to inspect items.
  - Library Progress:
    - `point.Point` grew a SumSQ method
    - improved `view` rendering, now supports left/right alignment and floating in
      header/footer lines
    - `ecs.Relation` finally got `UpsertOne` and `UpsertMany` methods

### [Thirteen](../../tree/thirteen)

I've got a Great View, and so can you!

Today I focused completely on improving the view code, which has been limping
along for some time. Adding the action bar in Twelve really tapped it out on
complexity. So now I have a basic layout engine capable of placing boxes in any
combination of top/bottom/middle, left/right/center (maybe flush-left/right)
alignment. It works by way of a `Renderable` interface, which we'll see how
long it lasts... (I'm not entirely happy with the interface shape...)

Not only did I write a decent test for the layout engine (which turned out to
be tricky to get right), but my [ECS skeleton
app](../../tree/thirteen/internal/ecs/skeleton/main.go) now has much more
fleshed out demo code around the view. I'm guardedly optimistic that this is
starting to be something that someone other than myself might be able to use.

The view layer itself became much more generic; you could use it with my layout
engine, or with the stronger set of opinions called `view.HUD` (used to be
called `view.Context`, so that's progress I guess). The `HUD` provides a
conceit of a "header", "footer" and "scrolling capped log" on top of a world
map.

### [Fourteen](../../tree/fourteen)

Render(ableing) the world!

Today was mostly spent reworking the main game code in terms of the new
hud-based view system; by which I mean, today was mostly spent writing tests
for and debugging the new layout engine...

- ECS progres:
  - Relation.UpsertMany became more useful (where function, and ex-nihilo inserts)
  - UpsertOne now derefs its args to verify them

- View progress:
  - ejected hud into its own package
  - promoted main's prompt system into the internal hud package
  - Grid regained WriteString methods
  - many layout fixes, and many more test cases
  - exported {Min,Max}Int through moremath
  - the hud now trys to leave a gap between the world grid and the logs, rather
    than gratuitously running over

- Main Game Progress
  - ported to the new hud system, made everything Renderable (action bar, new
    shared prompt system, and body sumary)
  - AIs now re-evaluate their goals every time (with a 16x inertia bonus for
    the prior goal); this causes them to be much more aggressive about looting
    corpses
  - the spawn logic gained an "agro deficit" term, that discounts the
    `hp+damage` if there's not enough hate in the world (making it more likely
    that something will spawn if there's not enough unresolved conflict)
  - unified goal setting under an `UpsertMany` from it's prior "lookup, maybe
    delete, then insert if we didn't have one"

### [Fifteen](../../tree/fifteen)

Slayer of bugs, and restorer of HP.

By sheer weight of test cases, I have solved most (I shudder to think "all")
bugs in the layout view engine... in fact I'm fairly certain there are
remaining deficiencies, they just haven't been salient to my pursuits...
Anyhow, the layout engine no longer staircases horribly; and the code is almost
understandable, so that's not nothing...

The game itself made decent progress: there's a nice new body summary
visualization, a resting mechanic, and a healing mechanic (currently healing is
the only thing that uses resting, but they're actually two separate mechanics
fwiw.)

Full Notes:

- Game:
  - made the game persist after "game over" so that you can see what happened to you.
  - many fixes to the body system, including severing now works correctly!
  - new shiny body summary, that uses an ASCII doll.
  - axed so many log sites to stop spamming the console.
  - added a resting/healing system: once you rest to a full charge, you then can
    heal up to that many HP on one body part.
  - movement system smoothed out a bit, less surprises.
  - reified charge relations out of pending moves.

- View System:
  - allow hud logs to be horizontally truncated, rather than hiding.
  - added an integration test for hud logs.
  - fixed key handling.
  - so many more layout tests; they layout engine now has less bugs.
  - there's a new `Grid.Lines` utility method, useful for tests and debguging.

[es-beta]: http://entity-systems.wikidot.com/rdbms-beta

