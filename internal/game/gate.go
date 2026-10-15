package game

// RatedDoors is the rated-door count the prestige gate reads against
// Rules.CalibAfter: Meta.Rated, or the per-topic attempts when an older
// save counted more there. Text that talks about the gate uses it, so it
// says what the gate will do.
func RatedDoors(m *Meta) int { return rated(m) }
