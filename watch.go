package nirilayout

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"slices"
	"strings"
	"time"
)

var (
	watchFlag    = flag.Bool("watch", false, N("run as a background daemon that reactivates a working output when the active one disappears (cable unplug or boot without an external monitor), instead of showing the GUI"))
	fallbackFlag = flag.String("fallback", "", N("in watch mode, the name of the layout to apply when no output is active; if empty, one is auto-picked from the connected outputs"))
)

const (
	// watchPollInterval is the safety poll that runs independently of niri
	// events, so recovery still happens even if no event fires on a hotplug.
	watchPollInterval = 4 * time.Second
	// watchDebounce coalesces a burst of niri events into a single check.
	watchDebounce = 250 * time.Millisecond
	// watchEventRetry is the backoff before restarting the event-stream
	// subprocess after it exits or fails to start.
	watchEventRetry = 1 * time.Second
)

// Watch reports whether -watch was passed. Consumed by main to run the daemon
// instead of the GUI.
func Watch() bool { return *watchFlag }

// niriOutput is the subset of a `niri msg -j outputs` entry we care about. The
// command returns a JSON object keyed by connector name; an output is active
// (drawing a picture) exactly when its "logical" region is non-null. A
// connected-but-off output appears with logical == null; a disconnected output
// is absent from the map entirely.
type niriOutput struct {
	Logical *json.RawMessage `json:"logical"`
}

// niriOutputs queries niri for the current output state.
func niriOutputs() (map[string]niriOutput, error) {
	out, err := exec.Command("niri", "msg", "-j", "outputs").Output()
	if err != nil {
		return nil, fmt.Errorf("niri msg outputs: %w", err)
	}
	var outputs map[string]niriOutput
	if err := json.Unmarshal(out, &outputs); err != nil {
		return nil, fmt.Errorf("parsing niri outputs: %w", err)
	}
	return outputs, nil
}

// anyActive reports whether at least one output is currently drawing a picture.
func anyActive(outputs map[string]niriOutput) bool {
	for _, o := range outputs {
		if o.Logical != nil {
			return true
		}
	}
	return false
}

// connectedNames returns the set of connector names niri currently sees,
// whether on or off. A layout is only applicable if every output it enables is
// in this set.
func connectedNames(outputs map[string]niriOutput) map[string]bool {
	set := make(map[string]bool, len(outputs))
	for name := range outputs {
		set[name] = true
	}
	return set
}

// pickSafeLayout chooses the layout to apply when the screen is black.
//
// With an explicit fallback name it returns that layout (matched
// case-insensitively), trusting the user to have named one whose output is
// always available. Otherwise it auto-picks: among layouts whose enabled
// outputs are all connected, it prefers the one lighting up the most outputs,
// so a single laptop panel is used only when nothing better is plugged in.
//
// parseLayoutFromConfig already strips `off` outputs, so layout.Outputs holds
// exactly the outputs a layout enables.
func pickSafeLayout(layouts []Layout, connected map[string]bool, fallback string) (Layout, bool) {
	if fallback != "" {
		for _, l := range layouts {
			if strings.EqualFold(l.Name, fallback) {
				return l, true
			}
		}
		return Layout{}, false
	}

	best := -1
	var bestLayout Layout
	for _, l := range layouts {
		if len(l.Outputs) == 0 {
			continue
		}
		allConnected := true
		for _, o := range l.Outputs {
			if !connected[o.Name] {
				allConnected = false
				break
			}
		}
		if allConnected && len(l.Outputs) > best {
			best = len(l.Outputs)
			bestLayout = l
		}
	}
	return bestLayout, best >= 0
}

// RunWatch runs the recovery daemon until the process is killed. It reacts to
// niri events (fast path) and a periodic poll (safety net), and whenever no
// output is active it applies a safe layout so niri reloads and lights a screen
// back up.
func RunWatch(configDir string, layouts []Layout) error {
	fallback := strings.TrimSpace(*fallbackFlag)
	if fallback != "" {
		if _, ok := pickSafeLayout(layouts, nil, fallback); !ok {
			return fmt.Errorf("fallback layout %q not found; known layouts: %s", fallback, layoutNames(layouts))
		}
	}

	if len(layouts) == 0 {
		return fmt.Errorf("no layouts found in %s (nothing to recover to)", configDir)
	}

	log.Printf("nirilayout: watch mode started (poll every %s, fallback=%q)", watchPollInterval, fallback)

	trigger := make(chan struct{}, 1)
	go pollLoop(trigger)
	go watchEvents(trigger)

	// Check once at startup so a boot with no external monitor recovers even
	// before the first event or poll tick.
	recoverScreen(configDir, layouts, fallback)

	for range trigger {
		// Coalesce a burst of events into one check, and let niri settle.
		time.Sleep(watchDebounce)
		drain(trigger)
		recoverScreen(configDir, layouts, fallback)
	}
	return nil
}

// recoverScreen applies a safe layout when nothing is on screen. It is a no-op
// while any output is active, and never rewrites the layout that is already
// current, so a genuinely unrecoverable state (nothing connected) does not loop.
func recoverScreen(configDir string, layouts []Layout, fallback string) {
	outputs, err := niriOutputs()
	if err != nil {
		log.Printf("nirilayout: %v", err)
		return
	}
	if anyActive(outputs) {
		return
	}

	connected := connectedNames(outputs)
	safe, ok := pickSafeLayout(layouts, connected, fallback)
	if !ok {
		log.Printf("nirilayout: no active output and no applicable layout (connected: %s)", strings.Join(sortedKeys(connected), ", "))
		return
	}

	if CurrentLayoutPath(configDir, layouts) == safe.Path {
		// Already applied and still black — applying it again would not help.
		return
	}

	if err := WriteLayout(safe); err != nil {
		log.Printf("nirilayout: failed to apply safe layout %q: %v", safe.Name, err)
		return
	}
	log.Printf("nirilayout: no active output — applied safe layout %q", safe.Name)
}

// pollLoop fires the trigger on a fixed interval, independent of niri events.
func pollLoop(trigger chan<- struct{}) {
	t := time.NewTicker(watchPollInterval)
	defer t.Stop()
	for range t.C {
		notify(trigger)
	}
}

// watchEvents streams niri events and fires the trigger on each one, restarting
// the subprocess if it dies. The event content is irrelevant — any event is
// just a wakeup to re-check the authoritative `niri msg outputs`, since the
// event stream has no dedicated output-change event.
func watchEvents(trigger chan<- struct{}) {
	for {
		cmd := exec.Command("niri", "msg", "event-stream")
		stdout, err := cmd.StdoutPipe()
		if err == nil && cmd.Start() == nil {
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				notify(trigger)
			}
			cmd.Wait()
		}
		time.Sleep(watchEventRetry)
	}
}

// notify sends a non-blocking wakeup; a full channel already has a pending
// check, so dropping the signal loses nothing.
func notify(trigger chan<- struct{}) {
	select {
	case trigger <- struct{}{}:
	default:
	}
}

// drain empties any wakeups that queued up while a check was running.
func drain(trigger <-chan struct{}) {
	for {
		select {
		case <-trigger:
		default:
			return
		}
	}
}

func layoutNames(layouts []Layout) string {
	names := make([]string, 0, len(layouts))
	for _, l := range layouts {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
