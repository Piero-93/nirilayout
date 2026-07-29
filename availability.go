package nirilayout

import (
	"flag"
	"log"
)

var (
	dimUnavailableFlag  = flag.Bool("dim-unavailable", false, N("dim layouts whose outputs are not all connected and make them unselectable"))
	hideUnavailableFlag = flag.Bool("hide-unavailable", false, N("hide layouts whose outputs are not all connected; takes precedence over -dim-unavailable"))
)

// hiddenLayoutCount records how many layouts -hide-unavailable dropped, so an
// empty picker can tell "you have no layouts" apart from "none of your layouts
// fits what is plugged in". It stays zero unless layouts were actually hidden.
var hiddenLayoutCount int

// missingOutputs returns the outputs a layout enables that niri does not
// currently see. An empty result means the layout is applicable right now.
//
// parseLayoutFromConfig already strips `off` outputs and sorts the rest by
// connector name, so the result is deterministic and never mentions an output
// the layout would leave disabled anyway.
func missingOutputs(l Layout, connected map[string]bool) []string {
	var missing []string
	for _, o := range l.Outputs {
		if !connected[o.Name] {
			missing = append(missing, o.Name)
		}
	}
	return missing
}

// markUnavailable flags every layout that needs an output which is not
// connected, recording which ones are missing so the GUI can say why.
func markUnavailable(layouts []Layout, connected map[string]bool) {
	for i := range layouts {
		layouts[i].MissingOutputs = missingOutputs(layouts[i], connected)
		layouts[i].Unavailable = len(layouts[i].MissingOutputs) > 0
	}
}

// withoutUnavailable returns the layouts that can be applied right now, in the
// original order, and records how many were dropped.
func withoutUnavailable(layouts []Layout) []Layout {
	kept := make([]Layout, 0, len(layouts))
	for _, l := range layouts {
		if !l.Unavailable {
			kept = append(kept, l)
		}
	}
	hiddenLayoutCount = len(layouts) - len(kept)
	return kept
}

// resolveAvailability applies the -dim-unavailable / -hide-unavailable
// behaviour to layouts, given the set of connectors niri currently sees.
//
// Hiding takes precedence over dimming: a layout dropped from the picker cannot
// also be shown dimmed there, so passing both flags hides.
func resolveAvailability(layouts []Layout, dim, hide bool, connected map[string]bool) []Layout {
	if !dim && !hide {
		return layouts
	}
	markUnavailable(layouts, connected)
	if hide {
		return withoutUnavailable(layouts)
	}
	return layouts
}

// ApplyAvailability asks niri which outputs are connected and dims or hides the
// layouts that cannot be applied, according to the flags. Without either flag it
// returns the layouts untouched, without querying niri at all.
//
// A failed query is not fatal: without a trustworthy output list the safe
// behaviour is the old one, offering every layout, rather than dimming or hiding
// layouts that may well be applicable.
func ApplyAvailability(layouts []Layout) []Layout {
	dim, hide := *dimUnavailableFlag, *hideUnavailableFlag
	if !dim && !hide {
		return layouts
	}

	outputs, err := niriOutputs()
	if err != nil {
		log.Printf("nirilayout: %v; offering all layouts", err)
		return layouts
	}

	return resolveAvailability(layouts, dim, hide, connectedNames(outputs))
}
