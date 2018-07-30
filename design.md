# Prompts

What's the difference between an entity and a component?

Both just seem like aspects interested in handle identity / type.

Would like a generic component manager, with swappable storage; maybe using
reflection at first, maybe just manual boilerplate, eventually accelerated with
codegen.

What's a world? scope? domain? (uni)verse?

How to model ids and handles? slices of same? maps of same?

How about borrowing the simulation region concept from handmade hero, and
(un)compressing entities between arenas?

(De)serializable from the beginning to support replay under anansi/x/platform.

# Prior rounds of execs

- had a similar world piece
- had an explicit entity type system
- really leaned into an entity relation system
- most perf hotspots were circa entity selection / querying
- didn't have enough genericism / abstraction around component manager equiv
