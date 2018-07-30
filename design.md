# Prompts

What's the difference between an entity and a component?

Generic component manager, with swappable storage; using reflection at first,
later accelerated with codegen.

What's a world? scope? domain? (uni)verse?

How to model ids and handles? slices of same? maps of same?

How to build an event queueing system? what's the difference with that and
component observation by systems for subject lists?.

# Prior rounds of execs

- had a similar world piece
- had an explicit entity type system
- really leaned into an entity relation system
- most perf hotspots were circa entity selection / querying
- didn't have enough genericism / abstraction around component manager equiv
