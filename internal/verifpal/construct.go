/* SPDX-FileCopyrightText: © 2019-2020 Nadim Kobeissi <nadim@symbolic.software>
 * SPDX-License-Identifier: GPL-3.0-only */
// e7f38dcfcb1b02f4419c2e9e90efa017

package verifpal

import "fmt"

func constructKnowledgeMap(m *model, principals []string) *knowledgeMap {
	valKnowledgeMap := knowledgeMap{
		principals:     principals,
		constants:      []constant{},
		assigned:       []value{},
		creator:        []string{},
		knownBy:        [][]map[string]string{},
		unnamedCounter: 0,
	}
	g := constant{
		name:        "g",
		guard:       false,
		fresh:       false,
		declaration: "knows",
		qualifier:   "public",
	}
	n := constant{
		name:        "nil",
		guard:       false,
		fresh:       false,
		declaration: "knows",
		qualifier:   "public",
	}
	valKnowledgeMap.constants = append(valKnowledgeMap.constants, g)
	valKnowledgeMap.assigned = append(valKnowledgeMap.assigned, value{
		kind:     "constant",
		constant: g,
	})
	valKnowledgeMap.creator = append(valKnowledgeMap.creator, principals[0])
	valKnowledgeMap.knownBy = append(valKnowledgeMap.knownBy, []map[string]string{})
	for _, principal := range principals {
		valKnowledgeMap.knownBy[0] = append(
			valKnowledgeMap.knownBy[0],
			map[string]string{principal: principal},
		)
	}
	valKnowledgeMap.constants = append(valKnowledgeMap.constants, n)
	valKnowledgeMap.assigned = append(valKnowledgeMap.assigned, value{
		kind:     "constant",
		constant: n,
	})
	valKnowledgeMap.creator = append(valKnowledgeMap.creator, principals[0])
	valKnowledgeMap.knownBy = append(valKnowledgeMap.knownBy, []map[string]string{})
	for _, principal := range principals {
		valKnowledgeMap.knownBy[1] = append(
			valKnowledgeMap.knownBy[1],
			map[string]string{principal: principal},
		)
	}
	for _, blck := range m.blocks {
		switch blck.kind {
		case "principal":
			for _, expr := range blck.principal.expressions {
				switch expr.kind {
				case "knows":
					constructKnowledgeMapRenderKnows(&valKnowledgeMap, &blck, &expr)
				case "generates":
					constructKnowledgeMapRenderGenerates(&valKnowledgeMap, &blck, &expr)
				case "assignment":
					constructKnowledgeMapRenderAssignment(&valKnowledgeMap, &blck, &expr)
				}
			}
		case "message":
			constructKnowledgeMapRenderMessage(&valKnowledgeMap, &blck)
		}
	}
	return &valKnowledgeMap
}

func constructKnowledgeMapRenderKnows(valKnowledgeMap *knowledgeMap, blck *block, expr *expression) {
	for _, c := range expr.constants {
		i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		if i >= 0 {
			d1 := valKnowledgeMap.constants[i].declaration
			d2 := "knows"
			q1 := valKnowledgeMap.constants[i].qualifier
			q2 := expr.qualifier
			fresh := valKnowledgeMap.constants[i].fresh
			if d1 != d2 || q1 != q2 || fresh {
				errorCritical(fmt.Sprintf(
					"constant is known more than once and in different ways (%s)",
					prettyConstant(c),
				))
			}
			valKnowledgeMap.knownBy[i] = append(
				valKnowledgeMap.knownBy[i],
				map[string]string{blck.principal.name: blck.principal.name},
			)
		} else {
			c = constant{
				name:        c.name,
				guard:       c.guard,
				fresh:       false,
				declaration: "knows",
				qualifier:   expr.qualifier,
			}
			valKnowledgeMap.constants = append(valKnowledgeMap.constants, c)
			valKnowledgeMap.assigned = append(valKnowledgeMap.assigned, value{
				kind:     "constant",
				constant: c,
			})
			valKnowledgeMap.creator = append(valKnowledgeMap.creator, blck.principal.name)
			valKnowledgeMap.knownBy = append(valKnowledgeMap.knownBy, []map[string]string{})
			l := len(valKnowledgeMap.constants) - 1
			if expr.qualifier != "public" {
				continue
			}
			for _, principal := range valKnowledgeMap.principals {
				if principal != blck.principal.name {
					valKnowledgeMap.knownBy[l] = append(
						valKnowledgeMap.knownBy[l],
						map[string]string{principal: principal},
					)
				}
			}
		}
	}
}

func constructKnowledgeMapRenderGenerates(valKnowledgeMap *knowledgeMap, blck *block, expr *expression) {
	for _, c := range expr.constants {
		i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		if i >= 0 {
			errorCritical(fmt.Sprintf(
				"generated constant already exists (%s)",
				prettyConstant(c),
			))
			continue
		}
		c = constant{
			name:        c.name,
			guard:       c.guard,
			fresh:       true,
			declaration: "generates",
			qualifier:   "private",
		}
		valKnowledgeMap.constants = append(valKnowledgeMap.constants, c)
		valKnowledgeMap.assigned = append(valKnowledgeMap.assigned, value{
			kind:     "constant",
			constant: c,
		})
		valKnowledgeMap.creator = append(valKnowledgeMap.creator, blck.principal.name)
		valKnowledgeMap.knownBy = append(valKnowledgeMap.knownBy, []map[string]string{{}})
	}
}

func constructKnowledgeMapRenderAssignment(valKnowledgeMap *knowledgeMap, blck *block, expr *expression) {
	constants := sanityAssignmentConstants(expr.right, []constant{}, valKnowledgeMap)
	switch expr.right.kind {
	case "primitive":
		prim := primitiveGet(expr.right.primitive.name)
		if (len(expr.left) != prim.output) && (prim.output >= 0) {
			plural := ""
			output := fmt.Sprintf("%d", prim.output)
			if len(expr.left) > 1 {
				plural = "s"
			}
			if prim.output < 0 {
				output = "at least 1"
			}
			errorCritical(fmt.Sprintf(
				"primitive %s has %d output%s, expecting %s",
				prim.name, len(expr.left), plural, output,
			))
		}
		if expr.right.primitive.check {
			if !prim.check {
				errorCritical(fmt.Sprintf(
					"primitive %s is checked but does not support checking",
					prim.name,
				))
			}
		}
	}
	for _, c := range constants {
		i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		if i >= 0 {
			knows := false
			if valKnowledgeMap.creator[i] == blck.principal.name {
				knows = true
			}
			for _, m := range valKnowledgeMap.knownBy[i] {
				if _, ok := m[blck.principal.name]; ok {
					knows = true
				}
			}
			if !knows {
				errorCritical(fmt.Sprintf(
					"%s is using constant (%s) despite not knowing it",
					blck.principal.name,
					prettyConstant(c),
				))
			}
		} else {
			errorCritical(fmt.Sprintf(
				"constant does not exist (%s)",
				prettyConstant(c),
			))
		}
	}
	for i, c := range expr.left {
		if c.name == "_" {
			c.name = fmt.Sprintf("unnamed_%d", valKnowledgeMap.unnamedCounter)
			valKnowledgeMap.unnamedCounter = valKnowledgeMap.unnamedCounter + 1
		}
		ii := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		if ii >= 0 {
			errorCritical(fmt.Sprintf(
				"constant assigned twice (%s)",
				prettyConstant(c),
			))
		}
		c = constant{
			name:        c.name,
			guard:       c.guard,
			fresh:       false,
			declaration: "assignment",
			qualifier:   "private",
		}
		switch expr.right.kind {
		case "primitive":
			expr.right.primitive.output = i
		}
		valKnowledgeMap.constants = append(valKnowledgeMap.constants, c)
		valKnowledgeMap.assigned = append(valKnowledgeMap.assigned, expr.right)
		valKnowledgeMap.creator = append(valKnowledgeMap.creator, blck.principal.name)
		valKnowledgeMap.knownBy = append(valKnowledgeMap.knownBy, []map[string]string{{}})
	}
}

func constructKnowledgeMapRenderMessage(valKnowledgeMap *knowledgeMap, blck *block) {
	for _, c := range blck.message.constants {
		i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		if i < 0 {
			errorCritical(fmt.Sprintf(
				"%s sends unknown constant to %s (%s)",
				blck.message.sender,
				blck.message.recipient,
				prettyConstant(c),
			))
			continue
		}
		c = valKnowledgeMap.constants[i]
		senderKnows := false
		recipientKnows := false
		if valKnowledgeMap.creator[i] == blck.message.sender {
			senderKnows = true
		}
		for _, m := range valKnowledgeMap.knownBy[i] {
			if _, ok := m[blck.message.sender]; ok {
				senderKnows = true
			}
		}
		if valKnowledgeMap.creator[i] == blck.message.recipient {
			recipientKnows = true
		}
		for _, m := range valKnowledgeMap.knownBy[i] {
			if _, ok := m[blck.message.recipient]; ok {
				recipientKnows = true
			}
		}
		if !senderKnows {
			errorCritical(fmt.Sprintf(
				"%s is sending constant (%s) despite not knowing it",
				blck.message.sender,
				prettyConstant(c),
			))
		} else if recipientKnows {
			errorCritical(fmt.Sprintf(
				"%s is receiving constant (%s) despite already knowing it",
				blck.message.recipient,
				prettyConstant(c),
			))
		} else {
			valKnowledgeMap.knownBy[i] = append(
				valKnowledgeMap.knownBy[i],
				map[string]string{blck.message.recipient: blck.message.sender},
			)
		}
	}
}

func constructPrincipalStates(m *model, valKnowledgeMap *knowledgeMap) []*principalState {
	var valPrincipalStates []*principalState
	for _, principal := range valKnowledgeMap.principals {
		valPrincipalState := principalState{
			name:          principal,
			constants:     []constant{},
			assigned:      []value{},
			guard:         []bool{},
			known:         []bool{},
			sender:        []string{},
			wasRewritten:  []bool{},
			beforeRewrite: []value{},
			wasMutated:    []bool{},
			beforeMutate:  []value{},
		}
		for i, c := range valKnowledgeMap.constants {
			guard := false
			knows := false
			sender := valKnowledgeMap.creator[i]
			assigned := valKnowledgeMap.assigned[i]
			if valKnowledgeMap.creator[i] == principal {
				knows = true
			}
			for _, m := range valKnowledgeMap.knownBy[i] {
				if realSender, ok := m[principal]; ok {
					sender = realSender
					knows = true
				}
			}
			for _, blck := range m.blocks {
				switch blck.kind {
				case "message":
					for _, cc := range blck.message.constants {
						if ((c.name == cc.name) && cc.guard) &&
							((blck.message.recipient == principal) ||
								(valKnowledgeMap.creator[i] == principal)) {
							guard = true
						}
					}
				}
			}
			valPrincipalState.constants = append(valPrincipalState.constants, c)
			valPrincipalState.assigned = append(valPrincipalState.assigned, assigned)
			valPrincipalState.guard = append(valPrincipalState.guard, guard)
			valPrincipalState.known = append(valPrincipalState.known, knows)
			valPrincipalState.sender = append(valPrincipalState.sender, sender)
			valPrincipalState.creator = append(valPrincipalState.creator, valKnowledgeMap.creator[i])
			valPrincipalState.wasRewritten = append(valPrincipalState.wasRewritten, false)
			valPrincipalState.beforeRewrite = append(valPrincipalState.beforeRewrite, assigned)
			valPrincipalState.wasMutated = append(valPrincipalState.wasMutated, false)
			valPrincipalState.beforeMutate = append(valPrincipalState.beforeMutate, assigned)
		}
		valPrincipalStates = append(valPrincipalStates, &valPrincipalState)
	}
	return valPrincipalStates
}

func constructPrincipalStateClone(valPrincipalState *principalState) *principalState {
	valPrincipalStateClone := principalState{
		name:          valPrincipalState.name,
		constants:     make([]constant, len(valPrincipalState.constants)),
		assigned:      make([]value, len(valPrincipalState.assigned)),
		guard:         make([]bool, len(valPrincipalState.guard)),
		known:         make([]bool, len(valPrincipalState.known)),
		creator:       make([]string, len(valPrincipalState.creator)),
		sender:        make([]string, len(valPrincipalState.sender)),
		wasRewritten:  make([]bool, len(valPrincipalState.wasRewritten)),
		beforeRewrite: make([]value, len(valPrincipalState.beforeRewrite)),
		wasMutated:    make([]bool, len(valPrincipalState.wasMutated)),
		beforeMutate:  make([]value, len(valPrincipalState.beforeMutate)),
	}
	copy(valPrincipalStateClone.constants, valPrincipalState.constants)
	copy(valPrincipalStateClone.assigned, valPrincipalState.beforeRewrite)
	copy(valPrincipalStateClone.guard, valPrincipalState.guard)
	copy(valPrincipalStateClone.known, valPrincipalState.known)
	copy(valPrincipalStateClone.creator, valPrincipalState.creator)
	copy(valPrincipalStateClone.sender, valPrincipalState.sender)
	copy(valPrincipalStateClone.wasRewritten, valPrincipalState.wasRewritten)
	copy(valPrincipalStateClone.beforeRewrite, valPrincipalState.beforeRewrite)
	copy(valPrincipalStateClone.wasMutated, valPrincipalState.wasMutated)
	copy(valPrincipalStateClone.beforeMutate, valPrincipalState.beforeRewrite)
	/*
		for i := range valPrincipalStateClone.wasRewritten {
			valPrincipalStateClone.wasRewritten[i] = false
			valPrincipalStateClone.wasMutated[i] = false
		}
	*/
	return &valPrincipalStateClone
}

func constructAttackerState(active bool, m *model, valKnowledgeMap *knowledgeMap, verbose bool) *attackerState {
	valAttackerState := attackerState{
		active:      active,
		known:       []value{},
		wire:        []bool{},
		conceivable: []value{},
		mutatedTo:   [][]string{},
	}
	constructAttackerStatePopulate(m, valKnowledgeMap, &valAttackerState, verbose)
	return &valAttackerState
}

func constructAttackerStatePopulate(m *model, valKnowledgeMap *knowledgeMap, valAttackerState *attackerState, verbose bool) {
	for _, c := range valKnowledgeMap.constants {
		if c.qualifier == "public" {
			v := value{
				kind:     "constant",
				constant: c,
			}
			if sanityExactSameValueInValues(v, &valAttackerState.known) < 0 {
				valAttackerState.known = append(valAttackerState.known, v)
				valAttackerState.wire = append(valAttackerState.wire, false)
				valAttackerState.mutatedTo = append(valAttackerState.mutatedTo, []string{})
			}
		}
	}
	for _, blck := range m.blocks {
		switch blck.kind {
		case "message":
			constructAttackerStateRenderMessage(valKnowledgeMap, valAttackerState, &blck, verbose)
		}
	}
}

func constructAttackerStateRenderMessage(valKnowledgeMap *knowledgeMap, valAttackerState *attackerState, blck *block, verbose bool) {
	for _, c := range blck.message.constants {
		i := sanityGetKnowledgeMapIndexFromConstant(valKnowledgeMap, c)
		v := value{
			kind:     "constant",
			constant: valKnowledgeMap.constants[i],
		}
		if valKnowledgeMap.constants[i].qualifier == "private" {
			ii := sanityExactSameValueInValues(v, &valAttackerState.known)
			if ii >= 0 {
				valAttackerState.wire[ii] = true
			} else {
				if verbose {
					prettyMessage(fmt.Sprintf(
						"%s has sent %s to %s, rendering it public",
						blck.message.sender, prettyConstant(c), blck.message.recipient,
					), 0, 0, "analysis")
				}
				valAttackerState.known = append(valAttackerState.known, v)
				valAttackerState.wire = append(valAttackerState.wire, true)
				valAttackerState.mutatedTo = append(valAttackerState.mutatedTo, []string{})
			}
		}
	}
}