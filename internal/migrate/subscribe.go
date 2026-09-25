// Copyright 2026 Synadia Communications Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migrate

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

// subForm is the shape of a legacy subscribe call.
type subForm int

const (
	formCallback subForm = iota + 1
	formSync
	formChan
	formPull
)

const (
	noteDurable        = "legacy Unsubscribe and Drain deleted a durable consumer the library had created; the jetstream package keeps it"
	noteCreateOrUpdate = "the consumer is created with CreateOrUpdateConsumer, as MIGRATION.md does: legacy used an existing durable as it was when the options it set were compatible, CreateOrUpdateConsumer applies the code's full configuration"
	noteBoundOptions   = "options describing the bound consumer are dropped: the consumer exists on the server, and the jetstream package does not compare them"
)

// cfgField is one consumer config field an option sets.
type cfgField struct {
	name, value string
	pushOnly    bool
}

// subCall is a parsed legacy subscribe call.
type subCall struct {
	call     *ast.CallExpr
	sym      string
	form     subForm
	queue    bool
	subject  ast.Expr
	queueArg ast.Expr
	handler  ast.Expr
	ch       ast.Expr
	durArg   ast.Expr // PullSubscribe's durable argument
	fields   []cfgField
	// bindStream and bindConsumer come from nats.Bind or nats.BindStream.
	bindStream, bindConsumer string
	ordered, manualAck       bool
	ackNone, durable, maxAck bool
	ctx                      ast.Expr
	pushOnly                 []string
	handled                  map[*ast.Ident]bool
}

// parseSubscribe splits a subscribe call into its arguments and folds its
// options into consumer config fields.
func (p *planner) parseSubscribe(rw *rewriter, call *ast.CallExpr, sym string) *subCall {
	sc := &subCall{call: call, sym: sym, handled: make(map[*ast.Ident]bool)}
	markCallee(call, sc.handled)
	args := call.Args
	take := func() ast.Expr {
		if len(args) == 0 {
			return nil
		}
		a := args[0]
		args = args[1:]
		return a
	}
	sc.subject = take()
	switch strings.TrimPrefix(sym, "JetStream.") {
	case "Subscribe":
		sc.form, sc.handler = formCallback, take()
	case "QueueSubscribe":
		sc.form, sc.queue, sc.queueArg, sc.handler = formCallback, true, take(), take()
	case "SubscribeSync":
		sc.form = formSync
	case "QueueSubscribeSync":
		sc.form, sc.queue, sc.queueArg = formSync, true, take()
	case "ChanSubscribe":
		sc.form, sc.ch = formChan, take()
	case "ChanQueueSubscribe":
		sc.form, sc.queue, sc.queueArg, sc.ch = formChan, true, take(), take()
	case "PullSubscribe":
		sc.form, sc.durArg = formPull, take()
	}
	if call.Ellipsis.IsValid() {
		rw.guided("subscribe options are passed as a slice; fold each into the consumer config")
		return sc
	}
	info := rw.f.info()
	for _, o := range args {
		oc, ok := ast.Unparen(o).(*ast.CallExpr)
		if !ok {
			rw.guided("subscribe option %s is not a direct call of a legacy option", p.prog.text(o))
			continue
		}
		osym, ok := calleeKey(info, oc)
		if !ok {
			rw.guided("subscribe option %s is not a legacy option", p.prog.text(o))
			continue
		}
		markCallee(oc, sc.handled)
		e := table[osym]
		argText := func(i int) string {
			if i < len(oc.Args) {
				return p.exprText(rw, oc.Args[i], sc.handled)
			}
			return ""
		}
		switch {
		case e.Kind == Field:
			for _, fv := range e.Fields {
				v := fv.Value
				switch {
				case strings.Contains(v, "&$1"):
					if len(oc.Args) != 1 || !isPlainRef(oc.Args[0]) {
						rw.guided("%s takes a pointer in the jetstream config; assign the value to a variable first", osym)
					}
					v = strings.ReplaceAll(v, "&$1", "&"+argText(0))
				case strings.Contains(v, "$*"):
					var all []string
					for i := range oc.Args {
						all = append(all, argText(i))
					}
					v = strings.ReplaceAll(v, "$*", strings.Join(all, ", "))
				case strings.Contains(v, "$1"):
					v = strings.ReplaceAll(v, "$1", argText(0))
				}
				sc.fields = append(sc.fields, cfgField{name: fv.Name, value: v, pushOnly: e.Note == notePushOnly})
			}
			switch osym {
			case "Durable":
				sc.durable = true
			case "MaxAckPending":
				sc.maxAck = true
			case "AckNone":
				sc.ackNone = true
			}
			if e.Note == notePushOnly {
				sc.pushOnly = append(sc.pushOnly, "nats."+osym)
			}
		case osym == "Bind" && len(oc.Args) == 2:
			sc.bindStream, sc.bindConsumer = argText(0), argText(1)
		case osym == "BindStream" && len(oc.Args) == 1:
			sc.bindStream = argText(0)
		case osym == "OrderedConsumer":
			sc.ordered = true
		case osym == "ManualAck":
			sc.manualAck = true
		case osym == "SkipConsumerLookup":
		case (osym == "Context" || osym == "ContextOpt") && len(oc.Args) == 1:
			sc.ctx = oc.Args[0]
		case e.Kind == Unmapped:
			rw.unmapped(fmt.Sprintf("%s: %s", osym, e.Note))
		default:
			rw.guided("subscribe option %s: %s", osym, entryHint(osym, e))
		}
	}
	if sc.queue && sc.bindConsumer == "" && !sc.durable && sc.queueArg != nil {
		// Legacy uses the queue name as the durable name.
		sc.fields = append([]cfgField{{name: "Durable", value: p.exprText(rw, sc.queueArg, sc.handled)}}, sc.fields...)
		sc.durable = true
	}
	return sc
}

// classifySubscribe classifies a legacy subscribe call: its decisions,
// and the rewrite their answers (or defaults) lead to.
func (p *planner) classifySubscribe(c *classified) {
	s := c.s
	call := s.anchor.(*ast.CallExpr)
	e := table[s.sym]
	c.ref = e.Ref
	probe := &rewriter{prog: p.prog, f: s.file}
	sc := p.parseSubscribe(probe, call, s.sym)
	compID := ""
	if c.comp != nil {
		compID = c.comp.id
	}
	ask := func(pattern, def, reason string) *decision {
		d := &decision{pattern: pattern, def: def, reason: reason}
		d.choice, d.scope = p.answers.lookup(pattern, c.id, compID)
		c.decisions = append(c.decisions, d)
		return d
	}
	var target, ack, pushOnly, chanMax, shared *decision
	if sc.form != formPull && !sc.ordered {
		def, reason := "pull", "the jetstream package is built around pull consumers, which is why teams migrate"
		if sc.bindConsumer != "" {
			def, reason = "push", "the subscription binds an existing consumer, which legacy Subscribe requires to be a push consumer; pull would fail until the consumer is recreated"
		}
		target = ask(patSubscribeTarget, def, reason)
	}
	if sc.form == formCallback && !sc.manualAck && !sc.ackNone && !sc.ordered {
		ack = ask(patAck, "after-handler", "legacy wrapped the handler as h(m); m.Ack(), so acking after it returns keeps the behavior")
	}
	if target != nil && target.choice == "pull" && len(sc.pushOnly) > 0 {
		pushOnly = ask(patPushOnly, "drop", fmt.Sprintf("%s has no pull equivalent", strings.Join(sc.pushOnly, ", ")))
	}
	if sc.form == formChan && !sc.maxAck && !sc.ackNone {
		chanMax = ask(patChanMaxAck, "keep", "legacy set MaxAckPending to the channel capacity")
	}
	h := p.handlerOf(s.file, sc)
	if h != nil && h.shared {
		shared = ask(patSharedHandler, "split", "the handler also serves a core subscription, which keeps receiving *nats.Msg")
	}
	choices := func(override *decision, opt string) map[string]string {
		m := make(map[string]string)
		for _, d := range []*decision{target, ack, pushOnly, chanMax, shared} {
			if d == nil {
				continue
			}
			m[d.pattern] = d.effective()
			if d == override {
				m[d.pattern] = opt
			}
		}
		return m
	}
	// Per-option replacements.
	for _, d := range c.decisions {
		d.after = make(map[string]string)
		for _, o := range patterns[d.pattern].options {
			rw := &rewriter{prog: p.prog, f: s.file}
			rw.problems = append(rw.problems, probe.problems...)
			intents, _ := p.buildSubscribe(c, rw, sc, h, choices(d, o.ID))
			d.after[o.ID] = p.optionAfter(c, rw, intents)
		}
	}
	rw := &rewriter{prog: p.prog, f: s.file}
	rw.problems = append(rw.problems, probe.problems...)
	intents, summary := p.buildSubscribe(c, rw, sc, h, choices(nil, ""))
	c.summary = summary
	c.notes = append(c.notes, uniq(rw.notes)...)
	for _, d := range s.dependents {
		intents = append(intents, p.dependentRewrite(c, rw, d, sc.handled)...)
	}
	p.checkHandled(rw, s, sc.handled, intents)
	// Notes and follow-ups do not depend on the answers.
	if sc.durable || (sc.form == formPull && !isEmptyString(s.file.info(), sc.durArg)) {
		if sc.bindConsumer == "" {
			c.notes = append(c.notes, noteDurable)
		}
	}
	if sc.bindStream == "" {
		p.streamFollowUp(c, sc)
	}
	p.settle(c, rw, intents)
	switch hx := ast.Unparen(sc.handler).(type) {
	case *ast.FuncLit:
		for _, o := range p.nested(c, s.file, hx) {
			o.owner = c
		}
	case *ast.Ident:
		if h != nil && h.decl != nil && !h.shared {
			c.links = append(c.links, p.prog.key(h.obj))
			for _, o := range p.nested(c, h.file, h.decl) {
				o.owner = c
			}
		}
	}
}

// optionAfter renders the replacement one option leads to.
func (p *planner) optionAfter(c *classified, rw *rewriter, intents []intent) string {
	for _, pr := range rw.problems {
		return pr.class + ": " + pr.reason
	}
	return p.afterText(c, intents)
}

func isEmptyString(info *types.Info, e ast.Expr) bool {
	if e == nil {
		return true
	}
	tv, ok := info.Types[e]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.String && constant.StringVal(tv.Value) == ""
}

// handlerInfo describes a named callback handler.
type handlerInfo struct {
	decl   *ast.FuncDecl
	file   *srcFile
	obj    types.Object
	shared bool
}

// handlerOf resolves a callback handler naming a function of the loaded
// code, and reports whether a core subscription also uses it.
func (p *planner) handlerOf(f *srcFile, sc *subCall) *handlerInfo {
	id, ok := ast.Unparen(sc.handler).(*ast.Ident)
	if !ok {
		return nil
	}
	fn, ok := f.info().Uses[id].(*types.Func)
	if !ok || !p.prog.inLoaded(fn) {
		return nil
	}
	h := &handlerInfo{obj: fn}
	hf := p.prog.fileOf(fn.Pos())
	for _, d := range hf.ast.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && hf.info().Defs[fd.Name] != nil && p.prog.key(hf.info().Defs[fd.Name]) == p.prog.key(fn) {
			h.decl, h.file = fd, hf
		}
	}
	k := p.prog.key(fn)
	for _, file := range p.prog.files {
		info := file.info()
		ast.Inspect(file.ast, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee, _ := typeutil.Callee(info, call).(*types.Func)
			if callee == nil || callee.Pkg() == nil || callee.Pkg().Path() != string(natsapi.Core) {
				return true
			}
			if _, legacy := legacyKey(callee); legacy {
				return true
			}
			for _, a := range call.Args {
				if aid, ok := ast.Unparen(a).(*ast.Ident); ok && p.prog.key(info.Uses[aid]) == k {
					h.shared = true
				}
			}
			return true
		})
	}
	return h
}

// buildSubscribe builds the rewrite of a subscribe site under the given
// decision choices.
func (p *planner) buildSubscribe(c *classified, rw *rewriter, sc *subCall, h *handlerInfo, ch map[string]string) ([]intent, string) {
	s := c.s
	call := sc.call
	sel, _ := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	jsh, isHandle := p.recvHandle(rw, sel.X)
	if !isHandle || jsh == nil {
		if !isHandle {
			rw.guided("the subscription is not made on a legacy handle variable")
		}
		return nil, "subscription on an unthreaded handle"
	}
	target := ch[patSubscribeTarget]
	if ch[patPushOnly] == "push" {
		target = "push"
	}
	if target == "defer" {
		rw.guided("deferred by the subscribe-target answer: the subscription stays on the legacy API for now, and the component keeps its legacy handle")
		return nil, "subscription deferred"
	}
	ctx := ""
	if sc.ctx != nil {
		ctx = p.exprText(rw, sc.ctx, sc.handled)
	} else {
		ctx, _ = p.prog.ctxAt(s.file, call.Pos(), nil)
	}
	subject := p.exprText(rw, sc.subject, sc.handled)
	avoid := idents(call)
	names := struct{ stream, cons, err, handler, msg string }{
		fresh("stream", avoid), fresh("cons", avoid), fresh("err", avoid), fresh("handler", avoid), fresh("msg", avoid),
	}
	indent := p.prog.indentOf(call.Pos())
	in1, in2 := indent+"\t", indent+"\t\t"
	recv := &builder{}
	p.siblingRecv(recv, sel.X, jsh)
	b := &builder{}
	var extra []intent
	// The consumer config.
	fields := slices.Clone(sc.fields)
	if sc.form != formPull && !sc.ordered {
		if ch[patAck] == "none" {
			// The answer overrides an ack option (nats.AckAll, nats.AckExplicit).
			fields = slices.DeleteFunc(fields, func(f cfgField) bool { return f.name == "AckPolicy" })
			fields = append(fields, cfgField{name: "AckPolicy", value: "jetstream.AckNonePolicy"})
		}
		if sc.form == formChan && ch[patChanMaxAck] == "keep" {
			fields = append(fields, cfgField{name: "MaxAckPending", value: "cap(" + p.prog.text(sc.ch) + ")"})
		}
	}
	if target == "pull" || sc.form == formPull {
		var kept []cfgField
		for _, f := range fields {
			if !f.pushOnly {
				kept = append(kept, f)
			}
		}
		if len(kept) != len(fields) {
			rw.notes = append(rw.notes, "push-only options are dropped: a pull consumer has no equivalent")
		}
		fields = kept
	}
	cfgText := func(typ string, fs []cfgField, base string) string {
		var sb strings.Builder
		sb.WriteString("jetstream." + typ + "{\n")
		for _, f := range fs {
			sb.WriteString(base + "\t" + f.name + ": " + f.value + ",\n")
		}
		sb.WriteString(base + "}")
		return sb.String()
	}
	ret := func(zero string) string {
		return in1 + "if " + names.err + " != nil {\n" + in2 + "return " + zero + names.err + "\n" + in1 + "}\n"
	}
	// streamExpr writes the stream lookup when the subscription is
	// unbound, and returns the stream name expression.
	streamExpr := func(zero string) string {
		if sc.bindStream != "" {
			return sc.bindStream
		}
		b.add(in1, names.stream, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".StreamNameBySubject(", ctx, ", ", subject, ")\n", ret(zero))
		return names.stream
	}
	summary := ""
	switch {
	case sc.form == formPull:
		summary = "PullSubscribe becomes a jetstream.Consumer"
		var fs []cfgField
		if !sc.durable && !isEmptyString(s.file.info(), sc.durArg) {
			fs = append(fs, cfgField{name: "Durable", value: p.exprText(rw, sc.durArg, sc.handled)})
		}
		fs = append(fs, fields...)
		fs = append(fs, cfgField{name: "FilterSubject", value: subject})
		switch {
		case sc.bindConsumer != "":
			if len(fields) > 0 {
				rw.notes = append(rw.notes, noteBoundOptions)
			}
			b.embed(recv.String(), recv.marks).add(".Consumer(", ctx, ", ", sc.bindStream, ", ", sc.bindConsumer, ")")
		case sc.bindStream != "":
			rw.notes = append(rw.notes, noteCreateOrUpdate)
			b.embed(recv.String(), recv.marks).add(".CreateOrUpdateConsumer(", ctx, ", ", sc.bindStream, ", ", cfgText("ConsumerConfig", fs, indent), ")")
		default:
			rw.notes = append(rw.notes, noteCreateOrUpdate)
			b.add("func() (jetstream.Consumer, error) {\n")
			stream := streamExpr("nil, ")
			b.add(in1, "return ").embed(recv.String(), recv.marks).add(".CreateOrUpdateConsumer(", ctx, ", ", stream, ", ", cfgText("ConsumerConfig", fs, in1), ")\n", indent, "}()")
		}
		extra = append(extra, p.subscriptionUses(rw, sc, true)...)
	case sc.form == formSync || sc.form == formChan:
		kind := "sync"
		if sc.form == formChan {
			kind = "channel"
		}
		if sc.ordered {
			rw.guided("an ordered %s subscription becomes js.OrderedConsumer(ctx, stream, jetstream.OrderedConsumerConfig{...}) and its Messages() iterator; move the reading code onto it", kind)
		} else {
			rw.guided("a %s subscription becomes a consumer's Messages() iterator (MessagesContext.Next replaces NextMsg; a channel is fed from Next in a goroutine); rewrite the reading code around it", kind)
		}
		return nil, fmt.Sprintf("%s subscription becomes a Messages iterator", kind)
	case sc.ordered:
		summary = "ordered Subscribe becomes an ordered consumer's Consume"
		var ofs []cfgField
		filter := cfgField{name: "FilterSubjects", value: "[]string{" + subject + "}"}
		for _, f := range fields {
			if f.name == "FilterSubjects" {
				filter = f
				continue
			}
			if _, ok := p.prog.jsField("OrderedConsumerConfig", f.name); !ok {
				rw.guided("the ordered consumer has no %s setting", f.name)
				continue
			}
			ofs = append(ofs, f)
		}
		ofs = append([]cfgField{filter}, ofs...)
		b.add("func() (jetstream.ConsumeContext, error) {\n")
		stream := streamExpr("nil, ")
		b.add(in1, names.cons, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".OrderedConsumer(", ctx, ", ", stream, ", ", cfgText("OrderedConsumerConfig", ofs, in1), ")\n", ret("nil, "))
		p.consume(b, rw, c, sc, h, ch, names.cons, names.handler, names.msg, indent, false)
		b.add(indent, "}()")
		extra = append(extra, p.subscriptionUses(rw, sc, false)...)
	case target == "push":
		summary = "Subscribe becomes a push consumer's Consume"
		b.add("func() (jetstream.ConsumeContext, error) {\n")
		if sc.bindConsumer != "" {
			if len(fields) > 0 {
				rw.notes = append(rw.notes, noteBoundOptions)
			}
			b.add(in1, names.cons, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".PushConsumer(", ctx, ", ", sc.bindStream, ", ", sc.bindConsumer, ")\n", ret("nil, "))
		} else {
			if sc.queue {
				rw.guided("queue instances of a push consumer must share one DeliverSubject; set it explicitly in the config instead of a per-instance inbox")
			}
			rw.notes = append(rw.notes, noteCreateOrUpdate)
			fs := slices.Clone(fields)
			if !slices.ContainsFunc(fs, func(f cfgField) bool { return f.name == "DeliverSubject" }) {
				fs = append(fs, cfgField{name: "DeliverSubject", value: "nats.NewInbox()"})
			}
			if sc.queue {
				fs = append(fs, cfgField{name: "DeliverGroup", value: p.exprText(rw, sc.queueArg, sc.handled)})
			}
			fs = append(fs, cfgField{name: "FilterSubject", value: subject})
			stream := streamExpr("nil, ")
			b.add(in1, names.cons, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".CreateOrUpdatePushConsumer(", ctx, ", ", stream, ", ", cfgText("ConsumerConfig", fs, in1), ")\n", ret("nil, "))
		}
		p.consume(b, rw, c, sc, h, ch, names.cons, names.handler, names.msg, indent, true)
		b.add(indent, "}()")
		extra = append(extra, p.subscriptionUses(rw, sc, false)...)
	default:
		summary = "Subscribe becomes a pull consumer's Consume"
		b.add("func() (jetstream.ConsumeContext, error) {\n")
		if sc.bindConsumer != "" {
			if len(fields) > 0 {
				rw.notes = append(rw.notes, noteBoundOptions)
			}
			b.add(in1, names.cons, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".Consumer(", ctx, ", ", sc.bindStream, ", ", sc.bindConsumer, ")\n", ret("nil, "))
		} else {
			rw.notes = append(rw.notes, noteCreateOrUpdate)
			fs := append(slices.Clone(fields), cfgField{name: "FilterSubject", value: subject})
			stream := streamExpr("nil, ")
			b.add(in1, names.cons, ", ", names.err, " := ").embed(recv.String(), recv.marks).add(".CreateOrUpdateConsumer(", ctx, ", ", stream, ", ", cfgText("ConsumerConfig", fs, in1), ")\n", ret("nil, "))
		}
		p.consume(b, rw, c, sc, h, ch, names.cons, names.handler, names.msg, indent, true)
		b.add(indent, "}()")
		extra = append(extra, p.subscriptionUses(rw, sc, false)...)
	}
	intents := []intent{b.at(s.file, p.prog.offset(call.Pos()), p.prog.offset(call.End()))}
	return append(intents, extra...), summary
}

// consume writes `return cons.Consume(handler)`, with the handler
// rewritten for jetstream.Msg and wrapped to ack after it returns when the
// ack answer says so.
func (p *planner) consume(b *builder, rw *rewriter, c *classified, sc *subCall, h *handlerInfo, ch map[string]string, cons, hv, mv, indent string, acks bool) {
	in1, in2 := indent+"\t", indent+"\t\t"
	wrap := acks && !sc.manualAck && !sc.ackNone && ch[patAck] == "after-handler"
	if acks && ch[patAck] == "explicit" {
		rw.guided("ack explicitly: add %s.Ack() (or Nak, Term) on each path of the handler", mv)
	}
	var fn string // the handler expression
	switch hx := ast.Unparen(sc.handler).(type) {
	case *ast.FuncLit:
		text, marks, ok := p.handlerLit(rw, c, hx, sc.handled)
		if !ok {
			return
		}
		text, marks = reindentMarks(text, marks, "\t")
		if !wrap {
			b.add(in1, "return ", cons, ".Consume(").embed(text, marks).add(")\n")
			return
		}
		b.add(in1, hv, " := ").embed(text, marks).add("\n")
		fn = hv
	case *ast.Ident:
		switch {
		case h == nil || h.decl == nil:
			rw.guided("the handler %s is not a function the planner can rewrite; change its parameter to jetstream.Msg", hx.Name)
			return
		case h.shared && ch[patSharedHandler] == "adapter":
			rw.notes = append(rw.notes, "the adapter hands the handler a *nats.Msg built from the jetstream.Msg; acks and replies on it do not reach the server")
			b.add(in1, "return ", cons, ".Consume(func(", mv, " jetstream.Msg) {\n", in2, hx.Name, "(&nats.Msg{Subject: ", mv, ".Subject(), Reply: ", mv, ".Reply(), Header: ", mv, ".Headers(), Data: ", mv, ".Data()})\n")
			if wrap {
				b.add(in2, mv, ".Ack()\n")
			}
			b.add(in1, "})\n")
			return
		case h.shared:
			name := fresh(hx.Name+"JS", p.pkgNames(h.file))
			decl, marks, ok := p.handlerDecl(rw, c, h, name)
			if !ok {
				return
			}
			_, end := p.prog.lineSpan(h.decl.Pos(), h.decl.End())
			db := &builder{}
			db.add("\n").embed(decl, marks).add("\n")
			rw.extra = append(rw.extra, db.at(h.file, end, end))
			fn = name
		default:
			decl, marks, ok := p.handlerDecl(rw, c, h, hx.Name)
			if !ok {
				return
			}
			db := &builder{}
			db.embed(decl, marks)
			rw.extra = append(rw.extra, db.at(h.file, p.prog.offset(h.decl.Pos()), p.prog.offset(h.decl.End())))
			fn = hx.Name
		}
	default:
		rw.guided("the handler is not statically known; change its parameter to jetstream.Msg")
		return
	}
	if !wrap {
		b.add(in1, "return ", cons, ".Consume(", fn, ")\n")
		return
	}
	b.add(in1, "return ", cons, ".Consume(func(", mv, " jetstream.Msg) {\n", in2, fn, "(", mv, ")\n", in2, mv, ".Ack()\n", in1, "})\n")
}

// pkgNames returns the package-level names of a file's package.
func (p *planner) pkgNames(f *srcFile) map[string]bool {
	out := make(map[string]bool)
	for _, n := range f.pkg.Types.Scope().Names() {
		out[n] = true
	}
	return out
}

// handlerLit rewrites a handler literal for jetstream.Msg, with the
// sites nested in it.
func (p *planner) handlerLit(rw *rewriter, c *classified, lit *ast.FuncLit, handled map[*ast.Ident]bool) (string, []mark, bool) {
	intents, obj, ok := p.msgParam(rw, rw.f, lit.Type, lit.Body, handled)
	if !ok {
		return "", nil, false
	}
	more, ok := p.nestedIntents(rw, c, rw.f, lit, obj)
	if !ok {
		return "", nil, false
	}
	text, marks := p.prog.applyWithinMarks(lit, append(intents, more...))
	return text, marks, true
}

// handlerDecl rewrites a handler function for jetstream.Msg under name,
// with the sites nested in it.
func (p *planner) handlerDecl(rw *rewriter, c *classified, h *handlerInfo, name string) (string, []mark, bool) {
	hrw := &rewriter{prog: p.prog, f: h.file}
	intents, obj, ok := p.msgParam(hrw, h.file, h.decl.Type, h.decl.Body, make(map[*ast.Ident]bool))
	more, ok2 := p.nestedIntents(hrw, c, h.file, h.decl, obj)
	rw.problems = append(rw.problems, hrw.problems...)
	if !ok || !ok2 {
		return "", nil, false
	}
	intents = append(intents, more...)
	if name != h.decl.Name.Name {
		intents = append(intents, p.prog.replace(h.decl.Name, name))
	}
	text, marks := p.prog.applyWithinMarks(h.decl, intents)
	return text, marks, true
}

// nestedIntents returns the edits of the sites inside a handler, other
// than the message methods msgParam rewrites: each must be mechanical.
func (p *planner) nestedIntents(rw *rewriter, c *classified, f *srcFile, n ast.Node, msg types.Object) ([]intent, bool) {
	var out []intent
	ok := true
	for _, o := range p.nested(c, f, n) {
		if msg != nil && isMsgCall(f.info(), o.s.anchor, msg) {
			continue
		}
		if o.class != classMechanical {
			rw.guided("the handler contains %s (%s), which is %s", o.id, o.summary, o.class)
			ok = false
			continue
		}
		out = append(out, o.intents...)
	}
	return out, ok
}

// nested returns the sites inside n other than c.
func (p *planner) nested(c *classified, f *srcFile, n ast.Node) []*classified {
	var out []*classified
	for _, o := range p.cls {
		if o != c && o.s.file == f && o.s.anchor.Pos() >= n.Pos() && o.s.anchor.End() <= n.End() {
			out = append(out, o)
		}
	}
	return out
}

// isMsgCall reports whether a site is a method call on the message msg.
func isMsgCall(info *types.Info, anchor ast.Node, msg types.Object) bool {
	call, ok := anchor.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	return ok && info.Uses[id] == msg
}

// msgFields maps *nats.Msg fields to jetstream.Msg accessors.
var msgFields = map[string]string{"Subject": "Subject()", "Reply": "Reply()", "Data": "Data()", "Header": "Headers()"}

// msgParam rewrites a handler's *nats.Msg parameter to jetstream.Msg and
// every use of it in body.
func (p *planner) msgParam(rw *rewriter, f *srcFile, ft *ast.FuncType, body *ast.BlockStmt, handled map[*ast.Ident]bool) ([]intent, types.Object, bool) {
	info := f.info()
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) > 1 {
		rw.guided("the handler does not take a single *nats.Msg parameter")
		return nil, nil, false
	}
	param := ft.Params.List[0]
	out := []intent{p.prog.replace(param.Type, "jetstream.Msg")}
	if len(param.Names) == 0 || param.Names[0].Name == "_" {
		return out, nil, true
	}
	obj := info.Defs[param.Names[0]]
	ast.Inspect(body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || info.Uses[id] != obj {
			return true
		}
		path, _ := astutil.PathEnclosingInterval(f.ast, id.Pos(), id.End())
		sel, ok := parentOf(path, id).(*ast.SelectorExpr)
		if !ok || sel.X != id {
			rw.guided("the message %s is used as a *nats.Msg value", id.Name)
			return true
		}
		if acc, ok := msgFields[sel.Sel.Name]; ok {
			switch up := parentOf(path, sel).(type) {
			case *ast.AssignStmt:
				if slices.Contains(up.Lhs, ast.Expr(sel)) {
					rw.guided("the handler assigns to %s.%s", id.Name, sel.Sel.Name)
					return true
				}
			case *ast.UnaryExpr:
				if up.Op == token.AND {
					rw.guided("the handler takes the address of %s.%s", id.Name, sel.Sel.Name)
					return true
				}
			case *ast.IncDecStmt:
				rw.guided("the handler modifies %s.%s", id.Name, sel.Sel.Name)
				return true
			}
			out = append(out, p.prog.replace(sel.Sel, acc))
			return true
		}
		call, ok := parentOf(path, sel).(*ast.CallExpr)
		sym, legacy := legacyKey(info.Uses[sel.Sel])
		if !ok || call.Fun != sel || !legacy {
			rw.guided("jetstream.Msg has no %s", sel.Sel.Name)
			return true
		}
		handled[sel.Sel] = true
		var keep []string
		var ctx ast.Expr
		for _, a := range call.Args {
			if ac, ok := ast.Unparen(a).(*ast.CallExpr); ok {
				if asym, ok := calleeKey(info, ac); ok && (asym == "Context" || asym == "ContextOpt") && len(ac.Args) == 1 {
					markCallee(ac, handled)
					ctx = ac.Args[0]
					continue
				}
				if _, ok := calleeKey(info, ac); ok {
					markCallee(ac, handled)
					rw.guided("ack option %s has no jetstream counterpart", p.prog.text(ac))
					continue
				}
			}
			keep = append(keep, p.exprText(rw, a, handled))
		}
		method := sel.Sel.Name
		switch sym {
		case "Msg.AckSync":
			method = "DoubleAck"
			c := ""
			if ctx != nil {
				c = p.exprText(rw, ctx, handled)
			} else {
				c, _ = p.prog.ctxAt(f, call.Pos(), nil)
			}
			keep = append([]string{c}, keep...)
		case "Msg.Metadata":
			rw2 := &rewriter{prog: p.prog, f: f}
			pc := &classified{}
			p.resultContext(pc, rw2, call, fitResults(typeutil.Callee(info, call).Type().(*types.Signature).Results(), nil))
			if len(rw2.problems) > 0 || len(pc.links) > 0 {
				rw.guided("the metadata's type changes to *jetstream.MsgMetadata; review how it is used")
			}
		}
		if table[sym].Kind != Same && sym != "Msg.AckSync" {
			rw.guided("%s: %s", sym, entryHint(sym, table[sym]))
			return true
		}
		out = append(out, intent{file: f, start: p.prog.offset(call.Pos()), end: p.prog.offset(call.End()), text: id.Name + "." + method + "(" + strings.Join(keep, ", ") + ")"})
		return true
	})
	return out, obj, len(rw.problems) == 0
}

// subscriptionUses rewrites the uses of the variable a subscribe call
// defines: Drain and Unsubscribe statements; anything else needs review.
func (p *planner) subscriptionUses(rw *rewriter, sc *subCall, pull bool) (out []intent) {
	f := rw.f
	info := f.info()
	path, _ := astutil.PathEnclosingInterval(f.ast, sc.call.Pos(), sc.call.End())
	as, ok := parentOf(path, sc.call).(*ast.AssignStmt)
	if !ok {
		if _, isExpr := parentOf(path, sc.call).(*ast.ExprStmt); isExpr {
			return nil
		}
		rw.guided("the subscription is used directly; its type changes to a jetstream consumer or consume context")
		return nil
	}
	lhs, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		rw.guided("the subscription is stored in %s, whose type changes", p.prog.text(as.Lhs[0]))
		return nil
	}
	if lhs.Name == "_" {
		return nil
	}
	if as.Tok != token.DEFINE || info.Defs[lhs] == nil {
		rw.guided("the subscription is assigned to an existing *nats.Subscription variable %s, whose type changes", lhs.Name)
		return nil
	}
	obj := info.Defs[lhs]
	uses, deleted := 0, 0
	defer func() {
		if pull && uses > 0 && uses == deleted {
			// Nothing reads the consumer any more.
			out = append(out, p.prog.replace(lhs, "_"))
			fresh := false
			for _, l := range as.Lhs[1:] {
				if id, ok := l.(*ast.Ident); ok && info.Defs[id] != nil {
					fresh = true
				}
			}
			if !fresh {
				o := p.prog.offset(as.TokPos)
				out = append(out, intent{file: f, start: o, end: o + 2, text: "="})
			}
		}
	}()
	for id, u := range info.Uses {
		if u != obj {
			continue
		}
		uses++
		up, _ := astutil.PathEnclosingInterval(f.ast, id.Pos(), id.End())
		sel, ok := parentOf(up, id).(*ast.SelectorExpr)
		call, isCall := parentOf(up, sel).(*ast.CallExpr)
		if !ok || !isCall || call.Fun != sel || len(call.Args) != 0 {
			if ok && isCall {
				// Legacy calls on the subscription (Fetch) are the site's
				// dependents.
				if _, legacy := legacyKey(info.Uses[sel.Sel]); legacy {
					continue
				}
			}
			rw.guided("the subscription %s is used at %s in a way the planner does not rewrite", lhs.Name, p.prog.fset.Position(id.Pos()))
			continue
		}
		stmt := parentOf(up, call)
		switch stmt.(type) {
		case *ast.ExprStmt, *ast.DeferStmt:
		default:
			rw.guided("the result of %s.%s is used; the jetstream counterpart returns nothing", lhs.Name, sel.Sel.Name)
			continue
		}
		switch sel.Sel.Name {
		case "Drain", "Unsubscribe":
		default:
			rw.guided("%s.%s has no jetstream counterpart", lhs.Name, sel.Sel.Name)
			continue
		}
		if pull {
			// A pull consumer holds no subscription outside Fetch.
			s, e := p.prog.lineSpan(stmt.Pos(), stmt.End())
			out = append(out, intent{file: f, start: s, end: e, text: ""})
			deleted++
			continue
		}
		if sel.Sel.Name == "Unsubscribe" {
			out = append(out, p.prog.replace(sel.Sel, "Stop"))
		}
	}
	return out
}

// streamFollowUp adds the follow-up to name the stream statically, with
// the one stream config in the loaded code that covers the subject.
func (p *planner) streamFollowUp(c *classified, sc *subCall) {
	fu := FollowUp{Kind: "stream-name", Summary: "replace the runtime StreamNameBySubject lookup with the stream's name"}
	subj, ok := constString(c.s.file.info(), sc.subject)
	if ok {
		var found []streamDecl
		for _, sd := range p.streamDecls() {
			if slices.ContainsFunc(sd.subjects, func(pat string) bool { return subjectCovers(pat, subj) }) {
				found = append(found, sd)
			}
		}
		if len(found) == 1 {
			pos := p.pos(found[0].pos)
			fu.Summary = fmt.Sprintf("the stream is likely %q, created at %s:%d; name it instead of looking it up", found[0].name, pos.File, pos.Line)
			fu.Candidate = &pos
		}
	}
	c.followUps = append(c.followUps, fu)
}

type streamDecl struct {
	name     string
	subjects []string
	pos      token.Pos
}

// streamDecls returns the stream config literals of the loaded code with
// a constant name and constant subjects.
func (p *planner) streamDecls() []streamDecl {
	var out []streamDecl
	for _, f := range p.prog.files {
		info := f.info()
		ast.Inspect(f.ast, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			t, ok := types.Unalias(info.TypeOf(lit)).(*types.Named)
			if !ok || t.Obj().Name() != "StreamConfig" || t.Obj().Pkg() == nil {
				return true
			}
			if path := t.Obj().Pkg().Path(); path != string(natsapi.Core) && path != jsPath {
				return true
			}
			name, ok := constString(info, litField(lit, "Name"))
			if !ok {
				return true
			}
			sl, ok := ast.Unparen(litField(lit, "Subjects")).(*ast.CompositeLit)
			if !ok {
				return true
			}
			sd := streamDecl{name: name, pos: lit.Pos()}
			for _, e := range sl.Elts {
				if s, ok := constString(info, e); ok {
					sd.subjects = append(sd.subjects, s)
				}
			}
			out = append(out, sd)
			return true
		})
	}
	return out
}

func constString(info *types.Info, e ast.Expr) (string, bool) {
	if e == nil {
		return "", false
	}
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// subjectCovers reports whether every subject matching subj also matches
// the stream subject pattern pat.
func subjectCovers(pat, subj string) bool {
	pt, st := strings.Split(pat, "."), strings.Split(subj, ".")
	for i, t := range pt {
		if t == ">" {
			return len(st) > i
		}
		if i >= len(st) {
			return false
		}
		switch {
		case t == "*":
			if st[i] == ">" {
				return false
			}
		case t != st[i]:
			return false
		}
	}
	return len(pt) == len(st)
}
