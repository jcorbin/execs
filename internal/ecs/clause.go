package ecs

// TypeClause is a logical filter for ComponentTypes.  If All is non-0, then
// Test()s true only for types that have all of those type bits set.
// Similarly if Any non-0, then Test()s true only for types that have at least
// one of those type bits set.
type TypeClause struct {
	All ComponentType
	Any ComponentType
}

// Test returns true/or false based on above logic description.
func (tcl TypeClause) Test(t ComponentType) bool {
	if tcl.All != 0 && !t.All(tcl.All) {
		return false
	}
	if tcl.Any != 0 && !t.Any(tcl.Any) {
		return false
	}
	return true
}

// Clause is a convenience constructor.
func Clause(all, any ComponentType) TypeClause { return TypeClause{all, any} }

// All is a convenience constructor.
func All(t ComponentType) TypeClause { return TypeClause{All: t} }

// Any is a convenience constructor.
func Any(t ComponentType) TypeClause { return TypeClause{Any: t} }

// TODO: boolean logic methods?
