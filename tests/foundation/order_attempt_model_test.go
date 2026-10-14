// Purpose: MODEL_ONLY independent intent/atomic-commit oracle for frozen OA1 §6.
// Depends: Go standard library; order-attempt-v1 §§1–3; no production implementation.
// Used: fixed CI traces and testing.F; future system adapters can consume commands.
// Invariants: all-state identity, terminal monotonicity, atomic C2, subject cleanup,
// immutable owner/deadline, bounded expiry and inventory conservation.
// Limits: one tenant/store, two principals/tabs, unit stock, logical minutes; no
// dispatch/pay-link/legacy/retention model. This is never REAL_PG/product proof.
package foundation_test

import (
	"fmt"
	"testing"
)

const (
	oaAdmit byte = iota
	oaLock
	oaStage
	oaCommit
	oaCrash
	oaLoseAck
	oaReplay
	oaAbort
	oaTick
	oaSweep
	oaErase
	oaRotate
	oaCancel
	oaDiscardJob
	oaHoldSubjectBarrier
	oaReleaseSubjectBarrier
	oaHoldScope
	oaReleaseScope
	oaCommandCount
)

const oaModelFenceLock uint8 = 1

const (
	oaPending      = 1
	oaCommitted    = 2
	oaAborted      = 3
	oaEffects      = 8 // cart/quote, destination, order, source, claims, pay-entry, outbox, receipt
	oaInitialStock = 8
)

type oaModelCommand struct {
	op, actor, generation, key, body, arg byte
}

type oaModelReply struct {
	code                         string
	id, generation, owner, order int
}

type oaModelAttempt struct {
	id, actor, generation, key, body, owner, deadline, state, order int
	hold                                                            int
	subjects                                                        uint8
	effects                                                         [oaEffects]int // durable creation records; active resources use hold
	job, cancelled                                                  bool
	executionLocks                                                  uint8
}

type oaModelDraft struct {
	id, stages int
}

type oaIntentModel struct {
	now, count, stock      int
	generation, credential [2]int
	drafts                 [2]oaModelDraft
	attempts               [16]oaModelAttempt
	subjectBarriers        uint8
	scopeHeld              [2]bool
}

func oaNewIntentModel() oaIntentModel {
	return oaIntentModel{stock: oaInitialStock, credential: [2]int{100, 200}}
}

func oaReceipt(a oaModelAttempt) oaModelReply {
	code := "pending"
	if a.state == oaCommitted {
		code = "committed"
	} else if a.state == oaAborted {
		code = "aborted"
	}
	return oaModelReply{code, a.id, a.generation, a.owner, a.order}
}

func (m *oaIntentModel) settleAbort(a *oaModelAttempt) {
	a.state, a.subjects, a.job = oaAborted, 0, false
	m.drafts[a.actor] = oaModelDraft{}
}

func (m *oaIntentModel) step(c oaModelCommand) oaModelReply {
	actor, generation := int(c.actor%2), int(c.generation)
	key, body := int(c.key%16), int(c.body%4)
	reject := oaModelReply{code: "rejected"}
	if c.op == oaAdmit || c.op == oaReplay {
		// Global key lookup precedes generation/placement validation in every state.
		for i := 0; i < m.count; i++ {
			a := &m.attempts[i]
			if a.key == key {
				if a.actor != actor || a.body != body {
					return reject // exactly the same non-disclosing conflict
				}
				return oaReceipt(*a)
			}
		}
		if c.op == oaReplay || generation != m.generation[actor] || m.count == len(m.attempts) {
			return reject
		}
		for i := 0; i < m.count; i++ {
			if m.attempts[i].actor == actor && m.attempts[i].state == oaPending {
				return reject
			}
		}
		m.generation[actor]++
		m.count++
		a := oaModelAttempt{id: m.count, actor: actor, generation: m.generation[actor], key: key, body: body,
			owner: m.credential[actor], deadline: m.now + 15, state: oaPending, subjects: 1<<actor | 1<<(body+2), job: true}
		m.attempts[m.count-1] = a
		return oaReceipt(a)
	}
	if c.op == oaTick {
		m.now += int(c.arg % 32)
		return oaModelReply{code: "clock"}
	}
	if c.op == oaRotate {
		m.credential[actor]++
		return oaModelReply{code: "rotated"}
	}
	if c.op == oaCrash {
		m.drafts[actor] = oaModelDraft{}
		return oaModelReply{code: "unknown"}
	}
	if c.op == oaLoseAck {
		return oaModelReply{code: "unknown"} // observation cannot alter authority
	}
	if c.op == oaSweep || c.op == oaErase {
		for i := 0; i < m.count; i++ {
			a := &m.attempts[i]
			if a.state != oaPending || m.drafts[a.actor].id != 0 {
				continue // models a held execution fence; erasure must wait too
			}
			if c.op == oaErase {
				if a.subjects&(1<<(c.arg%6)) != 0 {
					m.settleAbort(a)
				}
			} else if m.now >= a.deadline {
				m.settleAbort(a)
			} else {
				a.job = true
			}
		}
		return oaModelReply{code: "swept"}
	}
	var a *oaModelAttempt
	for i := 0; i < m.count; i++ {
		if m.attempts[i].actor == actor && m.attempts[i].generation == generation {
			a = &m.attempts[i]
			break
		}
	}
	if a == nil {
		return reject
	}
	if c.op == oaCancel {
		if a.state != oaCommitted {
			return reject
		}
		if !a.cancelled {
			a.cancelled = true
			a.hold = 0
			m.stock++
		}
		return oaReceipt(*a)
	}
	if a.state != oaPending {
		return oaReceipt(*a)
	}
	d := &m.drafts[actor]
	switch c.op {
	case oaLock:
		if d.id != 0 {
			return reject
		}
		if m.now >= a.deadline {
			m.settleAbort(a)
		} else {
			*d = oaModelDraft{id: a.id}
		}
	case oaStage:
		if d.id != a.id {
			return reject
		}
		d.stages = min(oaEffects, d.stages+int(c.arg%9))
	case oaCommit:
		if d.id != a.id || d.stages != oaEffects {
			return reject
		}
		if m.now >= a.deadline || m.stock == 0 {
			m.settleAbort(a)
		} else {
			for i := range a.effects {
				a.effects[i] = 1
			}
			a.state, a.order, a.subjects, a.job = oaCommitted, a.id, 0, false
			a.hold = 1
			m.stock--
			*d = oaModelDraft{}
		}
	case oaAbort:
		if d.id != a.id {
			return reject
		}
		m.settleAbort(a)
	case oaDiscardJob:
		a.job = false
	default:
		return reject
	}
	return oaReceipt(*a)
}

// Observation histories survive crashes and terminal settlement. They are not
// read by the transition implementation, and supply its invariant oracle.
type oaModelObserver struct {
	identity       map[int][6]int
	terminal       map[int]oaModelReply
	lastGeneration [2]int
}

func (o *oaModelObserver) check(m oaIntentModel) error {
	pending := [2]int{}
	keys, intents, orders := map[int]bool{}, map[[2]int]bool{}, map[int]bool{}
	usedStock := 0
	for i := 0; i < m.count; i++ {
		a := m.attempts[i]
		identity := [6]int{a.actor, a.generation, a.key, a.body, a.owner, a.deadline}
		if old, ok := o.identity[a.id]; ok && old != identity {
			return fmt.Errorf("immutable identity changed: id=%d old=%v new=%v", a.id, old, identity)
		}
		o.identity[a.id] = identity
		intent := [2]int{a.actor, a.generation}
		if keys[a.key] || intents[intent] || a.generation > m.generation[a.actor] {
			return fmt.Errorf("duplicate key/generation or future attempt: %+v", a)
		}
		keys[a.key], intents[intent] = true, true
		wantEffect := 0
		switch a.state {
		case oaPending:
			pending[a.actor]++
			if a.subjects == 0 || a.order != 0 || a.hold != 0 {
				return fmt.Errorf("pending identity/subject violation: %+v", a)
			}
		case oaCommitted:
			wantEffect = 1
			if a.order == 0 || orders[a.order] {
				return fmt.Errorf("missing/duplicate order: %+v", a)
			}
			orders[a.order] = true
			wantHold := 1
			if a.cancelled {
				wantHold = 0
			}
			if a.hold != wantHold {
				return fmt.Errorf("active hold disagrees with cancellation: %+v", a)
			}
			usedStock += a.hold
		case oaAborted:
			if a.order != 0 || a.hold != 0 {
				return fmt.Errorf("aborted with order: %+v", a)
			}
		default:
			return fmt.Errorf("non-authoritative state: %d", a.state)
		}
		for _, effect := range a.effects {
			if effect != wantEffect {
				return fmt.Errorf("partial durable C2: %+v", a)
			}
		}
		if receipt, ok := o.terminal[a.id]; ok && receipt != oaReceipt(a) {
			return fmt.Errorf("terminal regression/pointer change: id=%d", a.id)
		}
		if a.state != oaPending {
			if a.subjects != 0 || a.job {
				return fmt.Errorf("terminal retained subjects/job: %+v", a)
			}
			o.terminal[a.id] = oaReceipt(a)
		}
	}
	for actor, generation := range m.generation {
		if generation < o.lastGeneration[actor] || pending[actor] > 1 {
			return fmt.Errorf("scope regressed or has multiple PENDING: actor=%d", actor)
		}
	}
	o.lastGeneration = m.generation
	if m.stock < 0 || m.stock+usedStock != oaInitialStock {
		return fmt.Errorf("stock conservation: available=%d held=%d", m.stock, usedStock)
	}
	return nil
}

// Separate pre/post oracle: state invariants alone cannot detect a valid-looking
// complete order written by an unfenced or stale-generation command.
func oaCheckModelTransition(before, after oaIntentModel, c oaModelCommand, reply oaModelReply) error {
	if reply.code == "rejected" && before != after {
		return fmt.Errorf("rejected command wrote state")
	}
	if c.op == oaLoseAck && before != after {
		return fmt.Errorf("acknowledgement loss changed authority")
	}
	for i := 0; i < before.count; i++ {
		a := before.attempts[i]
		if (c.op == oaAdmit || c.op == oaReplay) && a.key == int(c.key%16) {
			want := oaModelReply{code: "rejected"}
			if a.actor == int(c.actor%2) && a.body == int(c.body%4) {
				want = oaReceipt(a)
			}
			if reply != want || before != after {
				return fmt.Errorf("all-state replay/conflict changed or disclosed authority")
			}
		}
		if a.state != oaCommitted && after.attempts[i].state == oaCommitted {
			d := before.drafts[a.actor]
			if c.op != oaCommit || int(c.actor%2) != a.actor || int(c.generation) != a.generation ||
				d.id != a.id || d.stages != oaEffects || before.now >= a.deadline || before.stock <= 0 {
				return fmt.Errorf("commit lacked exact owner/generation/fence/deadline/resources")
			}
		}
	}
	if after.count > before.count {
		a := after.attempts[before.count]
		if c.op != oaAdmit || after.count != before.count+1 || int(c.generation) != before.generation[a.actor] ||
			a.generation != before.generation[a.actor]+1 || a.owner != before.credential[a.actor] || a.deadline != before.now+15 {
			return fmt.Errorf("admission bypassed generation/owner/deadline authority")
		}
	}
	return nil
}

func oaModelTrace(t testing.TB, commands []oaModelCommand) oaIntentModel {
	t.Helper()
	m := oaNewIntentModel()
	o := oaModelObserver{identity: map[int][6]int{}, terminal: map[int]oaModelReply{}}
	for i, c := range commands {
		before := m // value arrays give an independent durable/draft snapshot
		reply := m.step(c)
		if err := oaCheckModelTransition(before, m, c, reply); err != nil {
			t.Fatalf("MODEL_ONLY step=%d command=%+v: %v", i, c, err)
		}
		if err := o.check(m); err != nil {
			t.Fatalf("MODEL_ONLY step=%d command=%+v: %v", i, c, err)
		}
		// Replay every accepted key after every action, including later generations,
		// rotation/cancel/erase and crashes, without consulting mutable credentials.
		for j := 0; j < m.count; j++ {
			a := m.attempts[j]
			readBefore := m
			got := m.step(oaModelCommand{op: oaReplay, actor: byte(a.actor), key: byte(a.key), body: byte(a.body)})
			if got != oaReceipt(a) || m != readBefore {
				t.Fatalf("MODEL_ONLY exact all-state replay failed: step=%d id=%d reply=%+v", i, a.id, got)
			}
			for _, invalid := range []oaModelCommand{
				{op: oaReplay, actor: byte(1 - a.actor), key: byte(a.key), body: byte(a.body)},
				{op: oaAdmit, actor: byte(a.actor), key: byte(a.key), body: byte((a.body + 1) % 4)},
			} {
				if got := m.step(invalid); got != (oaModelReply{code: "rejected"}) || m != readBefore {
					t.Fatalf("MODEL_ONLY key conflict disclosed identity or changed state: step=%d reply=%+v", i, got)
				}
			}
		}
	}
	return m
}

func TestOrderAttemptModelSeeds(t *testing.T) {
	admit := oaModelCommand{op: oaAdmit}
	lock := oaModelCommand{op: oaLock, generation: 1}
	stage := oaModelCommand{op: oaStage, generation: 1, arg: oaEffects}
	commit := oaModelCommand{op: oaCommit, generation: 1}
	for stageCount := 0; stageCount <= oaEffects; stageCount++ {
		m := oaModelTrace(t, []oaModelCommand{admit, lock, {op: oaStage, generation: 1, arg: byte(stageCount)},
			{op: oaCrash}, commit, lock, stage, commit, {op: oaLoseAck}, {op: oaRotate}, commit})
		if m.count != 1 || m.attempts[0].state != oaCommitted || m.attempts[0].owner != 100 || m.stock != 7 {
			t.Fatalf("MODEL_ONLY crash-stage %d did not resume exactly once: %+v", stageCount, m)
		}
	}
	t.Run("cross_tab_terminal_generation_and_cancel", func(t *testing.T) {
		m := oaModelTrace(t, []oaModelCommand{admit, {op: oaAdmit, key: 1}, lock, stage, commit,
			{op: oaAdmit, key: 1}, {op: oaAdmit, generation: 1, key: 1}, commit,
			{op: oaCancel, generation: 1}, {op: oaCancel, generation: 1}, {op: oaAdmit, actor: 1, key: 2}})
		if m.count != 3 || m.generation != [2]int{2, 1} || m.stock != oaInitialStock {
			t.Fatalf("MODEL_ONLY explicit CAS or cancellation conservation failed: %+v", m)
		}
	})
	t.Run("discard_sweep_expiry_and_new_generation", func(t *testing.T) {
		m := oaModelTrace(t, []oaModelCommand{admit, {op: oaDiscardJob, generation: 1}, {op: oaSweep}})
		if !m.attempts[0].job || m.attempts[0].deadline != 15 {
			t.Fatal("MODEL_ONLY discarded job not recreated with original deadline")
		}
		m = oaModelTrace(t, []oaModelCommand{admit, {op: oaDiscardJob, generation: 1}, {op: oaTick, arg: 15},
			{op: oaSweep}, lock, stage, commit, {op: oaAdmit, generation: 1, key: 1}})
		if m.attempts[0].state != oaAborted || m.count != 2 || m.stock != oaInitialStock {
			t.Fatal("MODEL_ONLY expiry failed to fence stale execution/release scope")
		}
	})
	t.Run("deadline_crossed_inside_C2", func(t *testing.T) {
		m := oaModelTrace(t, []oaModelCommand{admit, lock, stage, {op: oaTick, arg: 15}, commit})
		if m.attempts[0].state != oaAborted || m.stock != oaInitialStock {
			t.Fatal("MODEL_ONLY deadline crossing committed effects")
		}
	})
	t.Run("linked_subject_erasure_waits_for_fence", func(t *testing.T) {
		m := oaModelTrace(t, []oaModelCommand{admit, lock, stage, {op: oaErase, arg: 2}, {op: oaCrash}, {op: oaErase, arg: 2}, lock, commit})
		if m.attempts[0].state != oaAborted || m.attempts[0].subjects != 0 {
			t.Fatal("MODEL_ONLY linked-subject erasure did not abort and shred")
		}
	})
}

func TestOrderAttemptModelOracleMutations(t *testing.T) {
	// These faults affect only independent value snapshots, never production code.
	// A green CI result demonstrates oracle sensitivity, not product mutation proof.
	pending := oaModelTrace(t, []oaModelCommand{{op: oaAdmit}, {op: oaLock, generation: 1}, {op: oaStage, generation: 1, arg: oaEffects}})
	committed := pending
	command := oaModelCommand{op: oaCommit, generation: 1}
	reply := committed.step(command)
	if err := oaCheckModelTransition(pending, committed, command, reply); err != nil {
		t.Fatalf("MODEL_ONLY positive oracle sentinel: %v", err)
	}
	positive := oaModelObserver{identity: map[int][6]int{}, terminal: map[int]oaModelReply{}}
	for _, state := range []oaIntentModel{pending, committed} {
		if err := positive.check(state); err != nil {
			t.Fatalf("MODEL_ONLY positive state sentinel: %v", err)
		}
	}
	for _, name := range []string{"drop_lock", "partial_C2", "pending_only_replay", "stale_generation", "replace_owner", "retain_subjects", "terminal_regression", "cancel_without_release"} {
		t.Run(name, func(t *testing.T) {
			before, after, c, r := pending, committed, command, reply
			switch name {
			case "drop_lock":
				before.drafts[0] = oaModelDraft{}
			case "partial_C2":
				after = pending
				after.attempts[0].effects[0] = 1
				c, r = oaModelCommand{op: oaStage, generation: 1}, oaReceipt(after.attempts[0])
			case "pending_only_replay":
				before, after = committed, committed
				c, r = oaModelCommand{op: oaReplay}, oaModelReply{code: "rejected"}
			case "stale_generation":
				c.generation = 0
			case "replace_owner":
				after.attempts[0].owner++
			case "retain_subjects":
				after.attempts[0].subjects = 1
			case "terminal_regression":
				before, after = committed, pending
				c, r = oaModelCommand{op: oaLoseAck}, oaModelReply{code: "unknown"}
			case "cancel_without_release":
				before, after = committed, committed
				c = oaModelCommand{op: oaCancel, generation: 1}
				r = after.step(c)
				after.stock--
			}
			o := oaModelObserver{identity: map[int][6]int{}, terminal: map[int]oaModelReply{}}
			if err := o.check(before); err != nil {
				t.Fatalf("MODEL_ONLY invalid fault baseline: %v", err)
			}
			err := oaCheckModelTransition(before, after, c, r)
			if err == nil {
				err = o.check(after)
			}
			if err == nil {
				t.Fatalf("MODEL_ONLY oracle missed injected %s", name)
			}
			t.Logf("MODEL_ONLY injected %s rejected: %v", name, err)
		})
	}
}

func FuzzOrderAttemptModel(f *testing.F) {
	// Six bytes per command: op, actor, expected/target generation, key, body, arg.
	for _, seed := range [][]byte{
		{}, {oaAdmit, 0, 0, 0, 0, 0},
		{oaAdmit, 0, 0, 0, 0, 0, oaLock, 0, 1, 0, 0, 0, oaStage, 0, 1, 0, 0, 8, oaCrash, 0, 0, 0, 0, 0},
		{oaAdmit, 0, 0, 0, 0, 0, oaAdmit, 1, 0, 0, 0, 0, oaTick, 0, 0, 0, 0, 15, oaSweep, 0, 0, 0, 0, 0},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 4096 {
			input = input[:4096]
		}
		commands := make([]oaModelCommand, 0, len(input)/6)
		for i := 0; i+5 < len(input); i += 6 {
			commands = append(commands, oaModelCommand{input[i] % oaCommandCount, input[i+1], input[i+2], input[i+3], input[i+4], input[i+5]})
		}
		// Explicit fair scheduling: crash releases dead executors, then an available
		// periodic sweeper settles every expired PENDING even with discarded jobs.
		commands = append(commands, oaModelCommand{op: oaCrash}, oaModelCommand{op: oaCrash, actor: 1},
			oaModelCommand{op: oaTick, arg: 15}, oaModelCommand{op: oaSweep})
		m := oaModelTrace(t, commands)
		for i := 0; i < m.count; i++ {
			if m.attempts[i].state == oaPending {
				t.Fatalf("MODEL_ONLY fair expiry sweep stranded attempt %d", m.attempts[i].id)
			}
		}
	})
}
