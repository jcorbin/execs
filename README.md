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
