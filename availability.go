package nirilayout

import (
	"flag"
	"log"
)

var dimUnavailableFlag = flag.Bool("dim-unavailable", false, N("dim layouts whose outputs are not all connected and make them unselectable"))

// DimUnavailable reports whether -dim-unavailable was passed. Consumed by main
// to decide whether to mark layouts before handing them to the GUI.
func DimUnavailable() bool { return *dimUnavailableFlag }

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

// MarkUnavailable asks niri which outputs are connected and marks the layouts
// that cannot be applied right now.
//
// A failed query is not fatal: without a trustworthy output list the safe
// behaviour is the old one, showing every layout as selectable, rather than
// dimming layouts that may well be applicable.
func MarkUnavailable(layouts []Layout) {
	outputs, err := niriOutputs()
	if err != nil {
		log.Printf("nirilayout: %v; showing all layouts as available", err)
		return
	}
	markUnavailable(layouts, connectedNames(outputs))
}
