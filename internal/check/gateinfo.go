package check

// weakDefinedSubject names the gate for a type defined as weak.Pointer[T] (gatewalk.go).
const weakDefinedSubject = "a type defined as weak.Pointer[T]"

// A Gate is one phase gate (gates.go) as gocpp explain shows it: what the checker rejects until
// a phase builds it, with the exact text of its message.
type Gate struct {
	Kind    string // "feature", "package", "member", "builtin" or "verb"
	Subject string // the feature, the import path, pkg.Name, the built-in function, the verb
	Message string // the error message, without "file:line:col: "
	Phase   int    // the phase that builds it; Never (99) for what gocpp never has
	Notes   string // what the gate covers (features and packages), plain text with `code`
}

// Gates returns every gate of the tables of gates.go, in the order of the tables. internal/explain
// builds its explanations of the gate messages from it, so a new row needs no second edit
// (TestMessagesCoverGates there checks it).
func Gates() []Gate {
	var out []Gate
	for i := range features {
		f := &features[i]
		if f.id == featPanicBasic {
			continue // an exemption: never reported
		}
		out = append(out, Gate{Kind: "feature", Subject: f.Name, Message: f.Message(), Phase: f.Phase, Notes: f.Notes})
	}
	// The generics gate that gatewalk.go reports for a type defined over weak.Pointer[T].
	out = append(out, Gate{Kind: "feature", Subject: weakDefinedSubject, Message: gateMessage(weakDefinedSubject, false, Never), Phase: Never,
		Notes: "A type declared over the intrinsic, like `type WP weak.Pointer[int]`: `weak.Pointer[T]` is a compiler intrinsic (a runtime class), so use it itself or an alias of it (`type A = weak.Pointer[int]`). Both work from Phase 03."})
	for i := range stdPackages {
		p := &stdPackages[i]
		subject := p.Path
		if p.Phase <= floorPhase {
			continue
		}
		out = append(out, Gate{Kind: "package", Subject: subject, Message: p.message(subject), Phase: p.Phase, Notes: p.Notes})
	}
	for i := range stdMembers {
		m := &stdMembers[i]
		out = append(out, Gate{Kind: "member", Subject: m.Pkg + "." + m.Name, Message: m.message(), Phase: m.Phase})
	}
	for i := range builtins {
		b := &builtins[i]
		out = append(out, Gate{Kind: "builtin", Subject: b.Name, Message: b.message(), Phase: b.Phase})
	}
	for _, v := range formatVerbs {
		out = append(out, Gate{Kind: "verb", Subject: "%" + string(v.Verb), Message: verbMessage(v.Verb, v.Phase), Phase: v.Phase})
	}
	return out
}
